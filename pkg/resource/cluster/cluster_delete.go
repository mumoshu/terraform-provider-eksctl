package cluster

import (
	"bytes"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource"
	"log"
	"os/exec"
)

func deleteCluster(d *schema.ResourceData) error {
	log.Printf("[DEBUG] deleting eksctl cluster with id %q", d.Id())

	set, err := PrepareClusterSet(d)
	if err != nil {
		return err
	}

	cluster := set.Cluster

	args := []string{
		"delete",
		"cluster",
		"-f", "-",
		"--wait",
	}

	if err := doDeleteKubernetesResourcesBeforeDestroy(cluster, d.Id()); err != nil {
		return err
	}

	cmd := exec.Command(cluster.EksctlBin, args...)

	cmd.Stdin = bytes.NewReader(set.ClusterConfig)

	if err := resource.Delete(cmd, d); err != nil {
		return err
	}

	clusterName := getClusterName(cluster, d.Id())

	if err := deleteVPCResourceTags(cluster, clusterName); err != nil {
		return err
	}

	return nil
}

