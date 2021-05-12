package cluster

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/api"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/tfsdk"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func (m *Manager) createCluster(d *schema.ResourceData) (*ClusterSet, error) {
	id := newClusterID()

	log.Printf("[DEBUG] creating eksctl cluster with id %q", id)

	set, err := m.PrepareClusterSet(d, id)
	if err != nil {
		return nil, err
	}

	cluster := set.Cluster

	ctx := mustNewContext(cluster)

	if err := createVPCResourceTags(cluster, set.ClusterName); err != nil {
		return nil, err
	}

	cmd, err := newEksctlCommandWithAWSProfile(cluster, "create", "cluster", "-f", "-")
	if err != nil {
		return nil, fmt.Errorf("creating eksctl-create command: %w", err)
	}

	cmd.Stdin = bytes.NewReader(set.ClusterConfig)

	if err := ctx.Create(cmd, d, id); err != nil {
		return nil, fmt.Errorf("running `eksctl create cluster`: %w: USED CLUSTER CONFIG:\n%s", err, string(set.ClusterConfig))
	}

	if err := doWriteKubeconfig(ctx, d, string(set.ClusterName), cluster.Region); err != nil {
		return nil, err
	}

	if err := doApplyKubernetesManifests(ctx, cluster, id); err != nil {
		return nil, err
	}

	if err := doAttachAutoScalingGroupsToTargetGroups(ctx, set); err != nil {
		return nil, err
	}

	if err := doCheckPodsReadiness(ctx, cluster, id); err != nil {
		return nil, err
	}

	if err := createIAMIdentityMapping(ctx, d, cluster); err != nil {
		return nil, err
	}

	return set, nil
}

func (m *Manager) doPlanKubeconfig(d *tfsdk.DiffReadWrite) error {
	var path string

	if v := d.Get(KeyKubeconfigPath); v != nil {
		path = v.(string)
	}

	if path == "" {
		d.SetNewComputed(KeyKubeconfigPath)
	}

	return nil
}

func doWriteKubeconfig(ctx *sdk.Context, d api.ReadWrite, clusterName, region string) error {
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

	cmd, err := newEksctlCommandFromResourceWithRegionAndProfile(d, "utils", "write-kubeconfig", "--cluster", clusterName)
	if err != nil {
		return fmt.Errorf("creating eksctl-utils-write-kubeconfig command: %w", err)
	}

	cmd.Env = append(cmd.Env, os.Environ()...)
	cmd.Env = append(cmd.Env, "KUBECONFIG="+path)

	if out, err := ctx.Run(cmd); err != nil {
		return fmt.Errorf("failed running %s %s: %vw: COMBINED OUTPUT:\n%s", cmd.Path, strings.Join(cmd.Args, " "), err, out.Output)
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

func createIAMIdentityMapping(ctx *sdk.Context, d api.ReadWrite, cluster *Cluster) error {
	iams, err := runGetIAMIdentityMapping(ctx, d, cluster)
	if err != nil {
		return fmt.Errorf("can not get iamidentitymapping from eks cluster: %w", err)
	}

	if len(iams) == 0 {
		log.Printf("no data from eksctl get iamidentitymapping")
	} else {
		if err := d.Set(KeyAWSAuthConfigMap, iams); err != nil {
			return fmt.Errorf("set aws-auth-configmap from iamidentitymapping : %w", err)
		}
	}

	if d.Get(KeyIAMIdentityMapping) != nil {
		values := d.Get(KeyIAMIdentityMapping).(*schema.Set)
		if err := runCreateIAMIdentityMapping(ctx, d, values, cluster); err != nil {
			return fmt.Errorf("creating create  iamidentitymapping command: %w", err)
		}

		if err := d.Set(KeyIAMIdentityMapping, values); err != nil {
			return fmt.Errorf("set  iamidentitymapping : %w", err)
		}
	}

	return nil
}

func runCreateIAMIdentityMapping(ctx *sdk.Context, d api.Getter, s *schema.Set, cluster *Cluster) error {
	values := s.List()
	for _, v := range values {
		ele := v.(map[string]interface{})
		args := []string{
			"create",
			"iamidentitymapping",
			"--cluster",
			cluster.Name,
			"--arn",
			ele["iamarn"].(string),
			"--username",
			ele["username"].(string),
		}

		g := ele["groups"]
		g2 := []string{}
		for _, v := range g.([]interface{}) {
			g2 = append(g2, "--group")
			g2 = append(g2, v.(string))
		}
		args = append(args, g2...)

		cmd, err := newEksctlCommandFromResourceWithRegionAndProfile(d, args...)

		if err != nil {
			return fmt.Errorf("creating create iamidentitymapping command: %w", err)
		}

		res, err := ctx.Run(cmd)
		if err != nil {
			return fmt.Errorf("running create iamidentitymapping command: %w", err)
		}

		log.Printf("eksctl creat iamidentitymapping response: %v", res)
	}
	return nil
}

func runDeleteIAMIdentityMapping(ctx *sdk.Context, d api.Getter, s *schema.Set, cluster *Cluster) error {
	values := s.List()
	for _, v := range values {
		ele := v.(map[string]interface{})
		args := []string{
			"delete",
			"iamidentitymapping",
			"--cluster",
			cluster.Name,
			"--arn",
			ele["iamarn"].(string),
		}

		cmd, err := newEksctlCommandFromResourceWithRegionAndProfile(d, args...)

		if err != nil {
			return fmt.Errorf("creating create iamidentitymapping command: %w", err)
		}

		res, err := ctx.Run(cmd)
		if err != nil {
			return fmt.Errorf("creating create-iamidentitymapping command: %w", err)
		}

		log.Printf("-----------res: %v", res)
	}
	return nil

}
