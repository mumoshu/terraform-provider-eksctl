package cluster

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"strings"
)

func (m *Manager) importCluster(d *schema.ResourceData) (*schema.ResourceData, error) {
	clusterName := d.Id()

	d.Set(KeyName, clusterName)
	d.Set(KeyBin, "eksctl")

	d.SetId(newClusterID())

	getCluster, err := newEksctlCommandFromResourceWithRegionAndProfile(d, "get", "cluster", "-o", "json", "--name", clusterName)
	if err != nil {
		return nil, fmt.Errorf("getting cluster %s:: %w", clusterName, err)
	}

	type resourceVpcConfig struct {
		VpcId string `json:"VpcId"`
	}

	type cluster struct {
		Arn               string            `json:"Arn"`
		Name              string            `json:"name"`
		ResourceVpcConfig resourceVpcConfig `json:"ResourcesVpcConfig"`
		Version           string            `json:"Version"`
	}

	var clusters []cluster

	if getClusterOut, err := getCluster.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed running %s %s: %vw: COMBINED OUTPUT:\n%s", getCluster.Path, strings.Join(getCluster.Args, " "), err, string(getClusterOut))
	} else if err := json.Unmarshal(getClusterOut, &clusters); err != nil {
		return nil, fmt.Errorf("parsing json: %w: INPUT:\n%s", err, string(getClusterOut))
	}

	var found *cluster

	var candidates []string

	for i := range clusters {
		c := clusters[i]
		if c.Name == clusterName {
			found = &c
			break
		}

		candidates = append(candidates, c.Name)
	}

	if found == nil {
		return nil, fmt.Errorf("found no cluster named %s in %v", clusterName, candidates)
	}

	expectedPrefix := "arn:aws:eks:"
	if !strings.HasPrefix(found.Arn, expectedPrefix) {
		return nil, fmt.Errorf("validating cluster arn: Arn %q must start with %q. This provider does not support this Arn yet", found.Arn, expectedPrefix)
	}

	colonSeparatedRegionAccountKindName := strings.TrimPrefix(found.Arn, expectedPrefix)

	regionAccountKindName := strings.Split(colonSeparatedRegionAccountKindName, ":")

	region := regionAccountKindName[0]

	d.Set(KeyVPCID, found.ResourceVpcConfig.VpcId)
	d.Set(KeyRegion, region)
	d.Set(KeyVersion, found.Version)

	return d, nil
}
