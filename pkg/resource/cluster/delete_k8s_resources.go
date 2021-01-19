package cluster

import (
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk"
	"golang.org/x/xerrors"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
)

func doDeleteKubernetesResourcesBeforeDestroy(ctx *sdk.Context, cluster *Cluster, id string) error {
	if len(cluster.DeleteKubernetesResourcesBeforeDestroy) == 0 {
		return nil
	}

	kubeconfig, err := ioutil.TempFile("", "terraform-provider-eksctl-kubeconfig-")
	if err != nil {
		return xerrors.Errorf("creating temp kubeconfig file: %w", err)
	}

	kubeconfigPath := kubeconfig.Name()

	if err := kubeconfig.Close(); err != nil {
		return xerrors.Errorf("writing kubeconfig: %w", err)
	}

	clusterName := cluster.Name + "-" + id

	writeKubeconfigCmd, err := newEksctlCommandWithAWSProfile(cluster, "utils", "write-kubeconfig", "--kubeconfig", kubeconfigPath, "--cluster", clusterName, "--region", cluster.Region)
	if err != nil {
		return xerrors.Errorf("initializing eksctl-utils-write-kubeconfig: %w", err)
	}

	if _, err := ctx.Run(writeKubeconfigCmd); err != nil {
		return xerrors.Errorf("running eksctl-utils-write-kubeconfig: %w", err)
	}

	for _, d := range cluster.DeleteKubernetesResourcesBeforeDestroy {
		kubectlCmd := exec.Command(cluster.KubectlBin, "delete", "-n", d.Namespace, d.Kind, d.Name)

		for _, env := range os.Environ() {
			if !strings.HasPrefix(env, "KUBECONFIG=") {
				kubectlCmd.Env = append(kubectlCmd.Env, env)
			}
		}

		kubectlCmd.Env = append(kubectlCmd.Env, "KUBECONFIG="+kubeconfigPath)

		if _, err := ctx.Run(kubectlCmd); err != nil {
			if strings.Contains(err.Error(), "not found") {
				log.Printf("Ignoring `kubectl delete` error %w. %s/%s/%s seems already deleted. Perhaps it is a stale cluster that was in the middle of deletion process?", err, d.Namespace, d.Kind, d.Name)
				continue
			}

			return xerrors.Errorf("running kubectl-delete: %w", err)
		}
	}

	return nil
}
