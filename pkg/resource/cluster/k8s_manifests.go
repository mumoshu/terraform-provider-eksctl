package cluster

import (
	"bytes"
	"fmt"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

func doApplyKubernetesManifests(ctx *sdk.Context, cluster *Cluster, id string) error {
	if len(cluster.Manifests) == 0 {
		return nil
	}

	kubeconfig, err := ioutil.TempFile("", "terraform-provider-eksctl-kubeconfig-")
	if err != nil {
		return err
	}

	kubeconfigPath := kubeconfig.Name()

	if err := kubeconfig.Close(); err != nil {
		return err
	}

	clusterName := cluster.Name + "-" + id

	writeKubeconfigCmd, err := newEksctlCommandWithAWSProfile(cluster, "utils", "write-kubeconfig", "--kubeconfig", kubeconfigPath, "--cluster", clusterName, "--region", cluster.Region)
	if err != nil {
		return fmt.Errorf("creating eksctl-utils-write-kubeconfig command: %w", err)
	}

	if _, err := ctx.Run(writeKubeconfigCmd); err != nil {
		return err
	}

	all := strings.Join(cluster.Manifests, "\n---\n")

	kubectlCmd := exec.Command(cluster.KubectlBin, "apply", "-f", "-")

	for _, env := range os.Environ() {
		if !strings.HasPrefix(env, "KUBECONFIG=") {
			kubectlCmd.Env = append(kubectlCmd.Env, env)
		}
	}

	kubectlCmd.Env = append(kubectlCmd.Env, "KUBECONFIG="+kubeconfigPath)

	kubectlCmd.Stdin = bytes.NewBufferString(all)

	if _, err := ctx.Run(kubectlCmd); err != nil {
		return err
	}

	return nil
}
