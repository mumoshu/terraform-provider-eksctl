package cluster

import (
	"bytes"
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func (m *Manager) updateCluster(d *schema.ResourceData) (*ClusterSet, error) {
	log.Printf("[DEBUG] updating eksctl cluster with id %q", d.Id())

	set, err := m.PrepareClusterSet(d)
	if err != nil {
		return nil, err
	}

	cluster, clusterConfig := set.Cluster, set.ClusterConfig

	ctx := mustNewContext(cluster)

	updateBy := func(args []string, harmlessErrors []string) func() error {
		return func() error {
			eksctlCmdToLog := fmt.Sprintf("eksctl-%s", strings.Join(args, "-"))

			args = append(args, "-f", "-")
			cmd, err := newEksctlCommandWithAWSProfile(cluster, args...)
			if err != nil {
				return fmt.Errorf("creating %s command: %w", eksctlCmdToLog, err)
			}

			cmd.Stdin = bytes.NewReader(clusterConfig)

			if r, err := ctx.Run(cmd); err != nil {
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

			if err := ctx.Update(cmd, d); err != nil {
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

			if err := ctx.Update(cmd, d); err != nil {
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

			if err := ctx.Update(cmd, d); err != nil {
				return fmt.Errorf("%v\n\nCLUSTER CONFIG:\n%s", err, string(clusterConfig))
			}

			return nil
		}
	}

	applyKubernetesManifests := func(id string) func() error {
		return func() error {
			return doApplyKubernetesManifests(ctx, cluster, id)
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

			if err := ctx.Update(cmd, d); err != nil {
				return fmt.Errorf("%v\n\nCLUSTER CONFIG:\n%s", err, string(clusterConfig))
			}

			return nil
		}
	}

	checkPodsReadiness := func(id string) func() error {
		return func() error {
			return doCheckPodsReadiness(ctx, cluster, id)
		}
	}

	writeKubeconfig := func() func() error {
		return func() error {
			return doWriteKubeconfig(ctx, d, string(set.ClusterName), cluster.Region)
		}
	}

	attachNodeGroupsToTargetGroups := func() func() error {
		return func() error {
			return doAttachAutoScalingGroupsToTargetGroups(ctx, set)
		}
	}

	id := d.Id()

	clusterName := string(set.ClusterName)
	harmlessFargateProfileCreationErrors := []string{
		fmt.Sprintf(`Error: no output "FargatePodExecutionRoleARN" in stack "eksctl-%s-cluster"`, clusterName),
		fmt.Sprintf(`Error: couldn't refresh role arn: no output "FargatePodExecutionRoleARN" in stack "eksctl-%s-cluster"`, clusterName),
	}

	drainNodegroup := func() func() error {

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
				cmd, err := newEksctlCommandFromResourceWithRegionAndProfile(d, opt...)

				if err != nil {
					return fmt.Errorf("creating eksctl drain command: %w", err)
				}

				if err := ctx.Update(cmd, d); err != nil {
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

			if err := runCreateIAMIdentityMapping(ctx, d, b.(*schema.Set).Difference(a.(*schema.Set)), cluster); err != nil {
				return fmt.Errorf("CreateIAMIdentityMapping Error: %v", err)
			}

			if err := runDeleteIAMIdentityMapping(ctx, d, a.(*schema.Set).Difference(b.(*schema.Set)), cluster); err != nil {
				return fmt.Errorf("DeleteIAMIdentityMapping Error: %v", err)
			}

			return nil
		}
	}

	tasks := []func() error{
		// See https://eksctl.io/usage/cluster-upgrade/ for the cluster upgrade process
		updateBy([]string{"upgrade", "cluster", "--approve"}, nil),
		updateBy([]string{"utils", "update-kube-proxy", "--approve"}, nil),
		updateBy([]string{"utils", "update-aws-node", "--approve"}, nil),
		updateBy([]string{"utils", "update-coredns", "--approve"}, nil),
		createNew("nodegroup", []string{"--timeout 90m"}, nil),
		whenIAMWithOIDCEnabled(associateIAMOIDCProvider()),
		whenIAMWithOIDCEnabled(createNew("iamserviceaccount", []string{"--approve"}, nil)),
		createNew("fargateprofile", nil, harmlessFargateProfileCreationErrors),
		enableRepo(),
		drainNodegroup(),
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
			return nil, err
		}
	}

	return set, nil
}
