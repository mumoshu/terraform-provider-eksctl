package cluster

import (
	"bytes"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource"
	"log"
	"os/exec"
)

func createCluster(d *schema.ResourceData) (*ClusterSet, error) {
	id := newClusterID()

	log.Printf("[DEBUG] creating eksctl cluster with id %q", id)

	set, err := PrepareClusterSet(d, id)
	if err != nil {
		return nil, err
	}

	cluster := set.Cluster

	if err := createVPCResourceTags(cluster, set.ClusterName); err != nil {
		return nil, err
	}

	cmd := exec.Command(cluster.EksctlBin, "create", "cluster", "-f", "-")

	cmd.Stdin = bytes.NewReader(set.ClusterConfig)

	if err := resource.Create(cmd, d, id); err != nil {
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
