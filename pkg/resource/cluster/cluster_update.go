package cluster

import (
	"bytes"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource"
	"log"
	"strings"
)

func (m *Manager) updateCluster(d *schema.ResourceData) error {
	log.Printf("[DEBUG] updating eksctl cluster with id %q", d.Id())

	set, err := m.PrepareClusterSet(d)
	if err != nil {
		return err
	}

	cluster, clusterConfig := set.Cluster, set.ClusterConfig

	createNew := func(kind string, harmlessErrors []string) func() error {
		return func() error {
			cmd, err := newEksctlCommand(cluster, "create", kind, "-f", "-")
			if err != nil {
				return fmt.Errorf("creating eksctl-create command: %w", err)
			}

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

			cmd, err := newEksctlCommand(cluster, args...)
			if err != nil {
				return fmt.Errorf("creating eksctl-delete command: %w", err)
			}

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
			cmd, err := newEksctlCommand(cluster, "utils", "associate-iam-oidc-provider", "-f", "-", "--approve")
			if err != nil {
				return fmt.Errorf("creating eksctl-utils-associate-iam-oidc-provider command: %w", err)
			}
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
			cmd, err := newEksctlCommand(cluster, "enable", "repo", "-f", "-")
			if err != nil {
				return fmt.Errorf("creating eksctl-enable-repo command: %w", err)
			}
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

	writeKubeconfig := func() func() error {
		return func() error {
			return doWriteKubeconfig(d, string(set.ClusterName), cluster.Region)
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
		writeKubeconfig(),
	}

	for _, t := range tasks {
		if err := t(); err != nil {
			return err
		}
	}

	return nil
}
