package cluster

import (
	"fmt"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
)

func doCheckPodsReadiness(cluster *Cluster, id string) error {
	if len(cluster.CheckPodsReadinessConfigs) == 0 {
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

	writeKubeconfigCmd := exec.Command(cluster.EksctlBin, "utils", "write-kubeconfig", "--kubeconfig", kubeconfigPath, "--cluster", clusterName, "--region", cluster.Region)
	if _, err := resource.Run(writeKubeconfigCmd); err != nil {
		return err
	}

	for _, r := range cluster.CheckPodsReadinessConfigs {
		args := []string{"wait", "--namespace", r.namespace, "--for", "condition=ready", "pod",
			"--timeout", fmt.Sprintf("%ds", r.timeoutSec),
		}

		var matches []string
		for k, v := range r.labels {
			matches = append(matches, k+"="+v)
		}

		args = append(args, "-l", strings.Join(matches, ","))

		var selectorArgs []string

		args = append(args, selectorArgs...)

		kubectlCmd := exec.Command(cluster.KubectlBin, args...)

		for _, env := range os.Environ() {
			if !strings.HasPrefix(env, "KUBECONFIG=") {
				kubectlCmd.Env = append(kubectlCmd.Env, env)
			}
		}

		kubectlCmd.Env = append(kubectlCmd.Env, "KUBECONFIG="+kubeconfigPath)

		if _, err := resource.Run(kubectlCmd); err != nil {
			return err
		}
	}

	return nil
}
