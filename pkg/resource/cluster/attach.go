package cluster

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/awsclicompat"
	"log"
	"strings"
)

func doAttachAutoScalingGroupsToTargetGroups(set *ClusterSet) error {
	cfn := cloudformation.New(awsclicompat.NewSession(set.Cluster.Region))

	var stackSummaries []*cloudformation.StackSummary

	var nextToken *string

	for {
		res, err := cfn.ListStacks(&cloudformation.ListStacksInput{
			NextToken: nextToken,
			StackStatusFilter: aws.StringSlice(
				[]string{
					cloudformation.StackStatusCreateComplete,
					cloudformation.StackStatusUpdateComplete,
				},
			),
		})
		if err != nil {
			return fmt.Errorf("attaching autoscaling groups to target groups: %w", err)
		}

		stackSummaries = append(stackSummaries, res.StackSummaries...)

		nextToken = res.NextToken

		if nextToken == nil {
			break
		}
	}

	stackNamePrefix := fmt.Sprintf("eksctl-%s-nodegroup-", set.ClusterName)

	log.Printf("Finding stacks whose name is prefixd with %q from %d stack summaries", stackNamePrefix, len(stackSummaries))

	asSvc := autoscaling.New(awsclicompat.NewSession(set.Cluster.Region))

	for _, s := range stackSummaries {

		if strings.HasPrefix(*s.StackName, stackNamePrefix) {
			log.Printf("processing stack summary for %s", *s.StackName)

			var targetGroupARNS []*string

			ngName := strings.TrimPrefix(*s.StackName, stackNamePrefix)

			for _, l := range set.ListenerStatuses {
				for _, a := range l.ALBAttachments {
					if a.NodeGroupName == ngName {
						targetGroupARNS = append(targetGroupARNS, l.DesiredTG.TargetGroupArn)
					}
				}
			}

			if len(targetGroupARNS) == 0 {
				continue
			}

			res, err := cfn.DescribeStackResource(&cloudformation.DescribeStackResourceInput{
				LogicalResourceId: aws.String("NodeGroup"),
				StackName:         s.StackName,
			})
			if err != nil {
				return fmt.Errorf("describing stack resource for %s: %w", *s.StackName, err)
			}

			asgARN := *res.StackResourceDetail.PhysicalResourceId

			_, asErr := asSvc.AttachLoadBalancerTargetGroups(&autoscaling.AttachLoadBalancerTargetGroupsInput{
				AutoScalingGroupName: aws.String(asgARN),
				TargetGroupARNs:      targetGroupARNS,
			})
			if aerr, ok := asErr.(awserr.Error); ok {
				return fmt.Errorf("attaching load balancer target groups: Code %s: %w", aerr.Code(), asErr)
			} else if asErr != nil {
				return fmt.Errorf("attaching load balancer target groups: unexpected error: %w", asErr)
			}
		}
	}

	return nil
}
