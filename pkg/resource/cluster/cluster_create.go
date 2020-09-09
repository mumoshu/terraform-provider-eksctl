package cluster

import (
	"bytes"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

func (m *Manager) createCluster(d *schema.ResourceData) (*ClusterSet, error) {
	id := newClusterID()

	log.Printf("[DEBUG] creating eksctl cluster with id %q", id)

	set, err := m.PrepareClusterSet(d, id)
	if err != nil {
		return nil, err
	}

	cluster := set.Cluster

	if err := createVPCResourceTags(cluster, set.ClusterName); err != nil {
		return nil, err
	}

	cmd, err := newEksctlCommand(cluster, "create", "cluster", "-f", "-")
	if err != nil {
		return nil, fmt.Errorf("creating eksctl-create command: %w", err)
	}

	cmd.Stdin = bytes.NewReader(set.ClusterConfig)

	if err := resource.Create(cmd, d, id); err != nil {
		return nil, fmt.Errorf("running `eksctl create cluster: %w: USED CLUSTER CONFIG:\n%s", err, string(set.ClusterConfig))
	}

	if err := doWriteKubeconfig(d, string(set.ClusterName), cluster.Region); err != nil {
		return nil, err
	}

	if err := doApplyKubernetesManifests(cluster, id); err != nil {
		return nil, err
	}

	if err := doAttachAutoScalingGroupsToTargetGroups(set); err != nil {
		return nil, err
	}

	if err := doCheckPodsReadiness(cluster, id); err != nil {
		return nil, err
	}

	return set, nil
}

func (m *Manager) doPlanKubeconfig(d *DiffReadWrite) error {
	var path string

	if v := d.Get(KeyKubeconfigPath); v != nil {
		path = v.(string)
	}

	if path == "" {
		d.SetNewComputed(KeyKubeconfigPath)
	}

	return nil
}

func doWriteKubeconfig(d ReadWrite, clusterName, region string) error {
	var path string

	if v := d.Get(KeyKubeconfigPath); v != nil {
		path = v.(string)
	}

	if path == "" {
		kubeconfig, err := ioutil.TempFile(os.TempDir(), "tf-eksctl-kubeconfig")
		if err != nil {
			return fmt.Errorf("failed generating kubeconfig path: %w", err)
		}
		_ = kubeconfig.Close()

		path = kubeconfig.Name()

		d.Set(KeyKubeconfigPath, path)
	}

	cmd, err := newEksctlCommandFromResource(d, "utils", "write-kubeconfig", "--cluster", clusterName, "--region", region)
	if err != nil {
		return fmt.Errorf("creating eksctl-utils-write-kubeconfig command: %w", err)
	}

	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, "KUBECONFIG="+path)

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed running %s %s: %vw: COMBINED OUTPUT:\n%s", cmd.Path, strings.Join(cmd.Args, " "), err, string(out))
	}

	log.Printf("Ran `%s %s` with KUBECONFIG=%s", cmd.Path, strings.Join(cmd.Args, " "), path)

	kubectlBin := "kubectl"
	if v := d.Get(KeyKubectlBin); v != nil {
		s := v.(string)
		if s != "" {
			kubectlBin = s
		}
	}

	retries := 5
	retryDelay := 5 * time.Second
	for i := 0; i < retries; i++ {
		kubectlVersion := exec.Command(kubectlBin, "version")
		kubectlVersion.Env = append(cmd.Env, os.Environ()...)
		kubectlVersion.Env = append(cmd.Env, "KUBECONFIG="+path)

		out, err := kubectlVersion.CombinedOutput()
		if err == nil {
			break
		}

		log.Printf("Retrying kubectl version error with KUBECONFIG=%s: %v: COMBINED OUTPUT:\n%s", path, err, string(out))
		time.Sleep(retryDelay)
	}

	return nil
}
