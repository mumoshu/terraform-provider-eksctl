package cluster

import (
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
)

func doDeleteKubernetesResourcesBeforeDestroy(cluster *Cluster, id string) error {
	if len(cluster.DeleteKubernetesResourcesBeforeDestroy) == 0 {
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

	for _, d := range cluster.DeleteKubernetesResourcesBeforeDestroy {
		kubectlCmd := exec.Command(cluster.KubectlBin, "delete", "-n", d.Namespace, d.Kind, d.Name)

		for _, env := range os.Environ() {
			if !strings.HasPrefix(env, "KUBECONFIG=") {
				kubectlCmd.Env = append(kubectlCmd.Env, env)
			}
		}

		kubectlCmd.Env = append(kubectlCmd.Env, "KUBECONFIG="+kubeconfigPath)

		if _, err := resource.Run(kubectlCmd); err != nil {
			if strings.Contains(err.Error(), "not found") {
				log.Printf("Ignoring `kubectl delete` error %w. %s/%s/%s seems already deleted. Perhaps it is a stale cluster that was in the middle of deletion process?", err, d.Namespace, d.Kind, d.Name)
				continue
			}
			return err
		}
	}

	return nil
}

