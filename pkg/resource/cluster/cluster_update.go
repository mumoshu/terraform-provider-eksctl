package cluster

import (
	"bytes"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource"
	"log"
	"os/exec"
	"strings"
)

func updateCluster(d *schema.ResourceData) error {
	log.Printf("[DEBUG] updating eksctl cluster with id %q", d.Id())

	set, err := PrepareClusterSet(d)
	if err != nil {
		return err
	}

	cluster, clusterConfig := set.Cluster, set.ClusterConfig

	createNew := func(kind string, harmlessErrors []string) func() error {
		return func() error {
			cmd := exec.Command(cluster.EksctlBin, "create", kind, "-f", "-")

			cmd.Stdin = bytes.NewReader(clusterConfig)

			if err := resource.Update(cmd, d); err != nil {
				lines := strings.Split(err.Error(), "\n")
				lastLine := lines[len(lines)-1]
				if lastLine == "" && len(lines) > 1 {
					lastLine = lines[len(lines)-2]
				}
				for _, h := range harmlessErrors {
					log.Printf("Checking if this is a harmless error while deleting missing %s: error is %q, checking against %q", kind, lastLine, h)

					if strings.HasPrefix(lastLine, h) {
						log.Printf("Ignoring harmless error while deleting missing %s: %v", kind, lastLine)

						return nil
					}
				}
				return fmt.Errorf("%v\n\nCLUSTER CONFIG:\n%s", err, string(clusterConfig))
			}

			return nil
		}
	}

	deleteMissing := func(kind string, extraArgs []string, harmlessErrors []string) func() error {
		return func() error {
			args := append([]string{"delete", kind, "-f", "-", "--only-missing"}, extraArgs...)

			cmd := exec.Command(cluster.EksctlBin, args...)

			cmd.Stdin = bytes.NewReader(clusterConfig)

			if err := resource.Update(cmd, d); err != nil {
				lines := strings.Split(err.Error(), "\n")
				lastLine := lines[len(lines)-1]
				if lastLine == "" && len(lines) > 1 {
					lastLine = lines[len(lines)-2]
				}
				for _, h := range harmlessErrors {
					log.Printf("Checking if this is a harmless error while deleting missing %s: error is %q, checking against %q", kind, lastLine, h)

					if strings.HasPrefix(lastLine, h) {
						log.Printf("Ignoring harmless error while deleting missing %s: %v", kind, lastLine)

						return nil
					}
				}

				return fmt.Errorf("%v\n\nCLUSTER CONFIG:\n%s", err, string(clusterConfig))
			}

			return nil
		}
	}

	associateIAMOIDCProvider := func() func() error {
		return func() error {
			cmd := exec.Command(cluster.EksctlBin, "utils", "associate-iam-oidc-provider", "-f", "-", "--approve")
			cmd.Stdin = bytes.NewReader(clusterConfig)

			if err := resource.Update(cmd, d); err != nil {
				return fmt.Errorf("%v\n\nCLUSTER CONFIG:\n%s", err, string(clusterConfig))
			}

			return nil
		}
	}

	applyKubernetesManifests := func(id string) func() error {
		return func() error {
			return doApplyKubernetesManifests(cluster, id)
		}
	}

	enableRepo := func() func() error {
		return func() error {
			cmd := exec.Command(cluster.EksctlBin, "enable", "repo", "-f", "-")
			cmd.Stdin = bytes.NewReader(clusterConfig)

			if err := resource.Update(cmd, d); err != nil {
				return fmt.Errorf("%v\n\nCLUSTER CONFIG:\n%s", err, string(clusterConfig))
			}

			return nil
		}
	}

	checkPodsReadiness := func(id string) func() error {
		return func() error {
			return doCheckPodsReadiness(cluster, id)
		}
	}

	attachNodeGroupsToTargetGroups := func() func() error {
		return func() error {
			return doAttachAutoScalingGroupsToTargetGroups(set)
		}
	}

	id := d.Id()

	harmlessFargateProfileCreationErrors := []string{
		fmt.Sprintf(`Error: no output "FargatePodExecutionRoleARN" in stack "eksctl-%s-%s-cluster"`, cluster.Name, id),
	}

	tasks := []func() error{
		createNew("nodegroup", nil),
		associateIAMOIDCProvider(),
		createNew("iamserviceaccount", nil),
		createNew("fargateprofile", harmlessFargateProfileCreationErrors),
		enableRepo(),
		deleteMissing("nodegroup", []string{"--drain"}, nil),
		deleteMissing("iamserviceaccount", nil, nil),
		// eksctl delete fargate profile doens't has --only-missing command
		//deleteMissing("fargateprofile", nil, []string{"Error: invalid Fargate profile: empty name"}),
		applyKubernetesManifests(id),
		attachNodeGroupsToTargetGroups(),
		checkPodsReadiness(id),
	}

	for _, t := range tasks {
		if err := t(); err != nil {
			return err
		}
	}

	return nil
}
