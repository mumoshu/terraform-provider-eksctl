package cluster

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func readCluster(d *schema.ResourceData) error {
	clusterNamePrefix := d.Get("name").(string)
	region := d.Get("region").(string)

	arns, err := getTargetGroupARNs(region, clusterNamePrefix)
	if err != nil {
		return fmt.Errorf("reading cluster: %w", err)
	}

	var v []interface{}

	for _, arn := range arns {
		v = append(v, arn)
	}

	if err := d.Set(KeyTargetGroupARNs, v); err != nil {
		return fmt.Errorf("setting resource data value for key %v: %w", KeyTargetGroupARNs, err)
	}

	return nil
}
