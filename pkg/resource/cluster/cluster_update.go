package cluster

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource"
)

func (m *Manager) updateCluster(d *schema.ResourceData) error {
	log.Printf("[DEBUG] updating eksctl cluster with id %q", d.Id())

	set, err := m.PrepareClusterSet(d)
	if err != nil {
		return err
	}

	cluster, clusterConfig := set.Cluster, set.ClusterConfig

	updateBy := func(args []string, harmlessErrors []string) func() error {
		return func() error {
			eksctlCmdToLog := fmt.Sprintf("eksctl-%s", strings.Join(args, "-"))

			args = append(args, "-f", "-")
			cmd, err := newEksctlCommandWithAWSProfile(cluster, args...)
			if err != nil {
				return fmt.Errorf("creating %s command: %w", eksctlCmdToLog, err)
			}

			cmd.Stdin = bytes.NewReader(clusterConfig)

			if r, err := resource.Run(cmd); err != nil {
				lines := strings.Split(err.Error(), "\n")
				lastLine := lines[len(lines)-1]
				if lastLine == "" && len(lines) > 1 {
					lastLine = lines[len(lines)-2]
				}
				for _, h := range harmlessErrors {
					log.Printf("Checking if this is a harmless error while running %s: error is %q, checking against %q", eksctlCmdToLog, lastLine, h)

					if strings.HasPrefix(lastLine, h) {
						log.Printf("Ignoring harmless error while running %s: %v", eksctlCmdToLog, lastLine)

						return nil
					}
				}

				var output string
				if r != nil {
					output = r.Output
				}

				return fmt.Errorf("%v\n\nCLUSTER CONFIG:\n%s\n\nOUTPUT:\n%s", err, string(clusterConfig), output)
			}

			return nil
		}
	}

	createNew := func(kind string, extraArgs []string, harmlessErrors []string) func() error {
		return func() error {
			args := []string{"create", kind, "-f", "-"}
			args = append(args, extraArgs...)
			cmd, err := newEksctlCommandWithAWSProfile(cluster, args...)
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

			cmd, err := newEksctlCommandWithAWSProfile(cluster, args...)
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
			cmd, err := newEksctlCommandWithAWSProfile(cluster, "utils", "associate-iam-oidc-provider", "-f", "-", "--approve")
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
			if g, err := cluster.GitOpsEnabled(); err != nil {
				return fmt.Errorf("reading git config from cluster.yaml: %w", err)
			} else if !g {
				return nil
			}

			cmd, err := newEksctlCommandWithAWSProfile(cluster, "enable", "repo", "-f", "-")
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

	clusterName := string(set.ClusterName)
	harmlessFargateProfileCreationErrors := []string{
		fmt.Sprintf(`Error: no output "FargatePodExecutionRoleARN" in stack "eksctl-%s-cluster"`, clusterName),
	}

	draineNodegroup := func() func() error {

		return func() error {

			args := []string{
				"drain",
				"nodegroup",
				"--cluster=" + clusterName,
				"-n",
			}
			nodegroups := d.Get(KeyDrainNodeGroups).(map[string]interface{})

			for k, v := range nodegroups {
				log.Printf("DRAIN    %v %v ", k, v)
				opt := append(args, string(k))

				if v == false {
					opt = append(opt, "--undo")
				}
				cmd, err := newEksctlCommandWithAWSProfile(cluster, opt...)

				if err != nil {
					return fmt.Errorf("creating eksctl drain command: %w", err)
				}

				if err := resource.Update(cmd, d); err != nil {
					return fmt.Errorf("Drain Error: %v", err)
				}
			}

			return nil
		}

	}

	whenIAMWithOIDCEnabled := func(f func() error) func() error {
		return func() error {
			iamWithOIDCEnabled, err := cluster.IAMWithOIDCEnabled()
			if err != nil {
				return fmt.Errorf("reading iam.withOIDC setting from cluster.yaml: %w", err)
			} else if !iamWithOIDCEnabled {
				return nil
			}

			return f()
		}
	}

	updateIAMIdentityMapping := func() func() error {
		return func() error {
			d.HasChange(KeyIAMIdentityMapping)
			a, b := d.GetChange(KeyIAMIdentityMapping)

			if err := runCreateIAMIdentityMapping(d, b.(*schema.Set).Difference(a.(*schema.Set)), cluster); err != nil {
				return fmt.Errorf("CreateIAMIdentityMapping Error: %v", err)
			}

			if err := runDeleteIAMIdentityMapping(d, a.(*schema.Set).Difference(b.(*schema.Set)), cluster); err != nil {
				return fmt.Errorf("DeleteIAMIdentityMapping Error: %v", err)
			}

			return nil
		}
	}

	tasks := []func() error{
		// See https://eksctl.io/usage/cluster-upgrade/ for the cluster upgrade process
		updateBy([]string{"upgrade", "cluster"}, nil),
		updateBy([]string{"utils", "update-kube-proxy"}, nil),
		updateBy([]string{"utils", "update-aws-node"}, nil),
		updateBy([]string{"utils", "update-coredns"}, nil),
		createNew("nodegroup", nil, nil),
		whenIAMWithOIDCEnabled(associateIAMOIDCProvider()),
		whenIAMWithOIDCEnabled(createNew("iamserviceaccount", []string{"--approve"}, nil)),
		createNew("fargateprofile", nil, harmlessFargateProfileCreationErrors),
		enableRepo(),
		draineNodegroup(),
		updateIAMIdentityMapping(),
		deleteMissing("nodegroup", []string{"--drain", "--approve"}, nil),
		whenIAMWithOIDCEnabled(deleteMissing("iamserviceaccount", []string{"--approve"}, nil)),
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
