package cluster

import (
	"bytes"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"log"
)

func (m *Manager) deleteCluster(d *schema.ResourceData) error {
	log.Printf("[DEBUG] deleting eksctl cluster with id %q", d.Id())

	set, err := m.PrepareClusterSet(d)
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

	ctx := mustNewContext(cluster)

	if err := doDeleteKubernetesResourcesBeforeDestroy(ctx, cluster, d.Id()); err != nil {
		return err
	}

	cmd, err := newEksctlCommandWithAWSProfile(cluster, args...)
	if err != nil {
		return fmt.Errorf("creating eksctl-delete command: %w", err)
	}

	cmd.Stdin = bytes.NewReader(set.ClusterConfig)

	if err := ctx.Delete(cmd); err != nil {
		return err
	}

	if err := deleteVPCResourceTags(cluster, set.ClusterName); err != nil {
		return err
	}

	// TODO Delete target groups
	// TODO Delete ALB listener rule

	return nil
}
