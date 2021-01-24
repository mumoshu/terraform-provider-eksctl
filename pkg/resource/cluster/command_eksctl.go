package cluster

import (
	"fmt"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/api"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/tfsdk"
	"os/exec"
)

func newEksctlCommandFromResourceWithRegionAndProfile(resource api.Getter, args ...string) (*exec.Cmd, error) {
	eksctlBin := resource.Get(KeyBin).(string)
	eksctlVersion := resource.Get(KeyEksctlVersion).(string)

	bin, err := sdk.PrepareExecutable(eksctlBin, "eksctl", eksctlVersion)
	if err != nil {
		return nil, fmt.Errorf("preparing eksctl binary: %w", err)
	}

	region, profile := tfsdk.GetAWSRegionAndProfile(resource)

	if region != "" {
		args = append(args, "--region", region)
	}

	if profile != "" {
		args = append(args, "--profile", profile)
	}

	cmd := exec.Command(*bin, args...)

	return cmd, nil
}

func newEksctlCommand(cluster *Cluster, args ...string) (*exec.Cmd, error) {
	eksctlBin, err := prepareEksctlBinary(cluster)
	if err != nil {
		return nil, fmt.Errorf("creating eksctl command: %w", err)
	}

	cmd := exec.Command(*eksctlBin, args...)

	return cmd, nil
}

// We don't add `--region` flag as this provider prefers metadata.region in cluster.yaml to specify the region
func newEksctlCommandWithAWSProfile(cluster *Cluster, args ...string) (*exec.Cmd, error) {
	_, profile := cluster.Region, cluster.Profile

	if profile != "" {
		args = append(args, "--profile", profile)
	}

	return newEksctlCommand(cluster, args...)
}
