package cluster

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/awsclicompat"
	"log"
)

const (
	TagKeyNodeGroupName     = "tf-eksctl/node-group"
	TagKeyClusterNamePrefix = "tf-eksctl/cluster"
)

func getTargetGroupARNs(region, clusterNamePrefixy string) ([]string, error) {
	api := resourcegroupstaggingapi.New(awsclicompat.NewSession(region))

	var token *string

	var arns []string

	for {
		log.Printf("getting tagged resources for %s", clusterNamePrefixy)

		res, err := api.GetResources(&resourcegroupstaggingapi.GetResourcesInput{
			PaginationToken:     token,
			ResourceTypeFilters: aws.StringSlice([]string{"elasticloadbalancing:targetgroup"}),
			TagFilters: []*resourcegroupstaggingapi.TagFilter{
				{
					Key:    aws.String(TagKeyClusterNamePrefix),
					Values: aws.StringSlice([]string{clusterNamePrefixy}),
				},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("getting tagged resources for %s: %w", clusterNamePrefixy, err)
		}

		for _, m := range res.ResourceTagMappingList {
			if arn := m.ResourceARN; arn != nil {
				arns = append(arns, *arn)
			}
		}

		token = res.PaginationToken
		if token == nil || *token == "" {
			break
		}
	}

	return arns, nil
}

func deleteTargetGroups(set *ClusterSet) error {
	elb := elbv2.New(awsclicompat.NewSession(set.Cluster.Region))

	for _, tgARN := range set.Cluster.TargetGroupARNs {
		log.Printf("Deleting target group %s for %s", tgARN, set.ClusterName)

		if _, err := elb.DeleteTargetGroup(&elbv2.DeleteTargetGroupInput{TargetGroupArn: aws.String(tgARN)}); err != nil {
			return fmt.Errorf("deleting target group %s for %s", tgARN, set.ClusterName)
		}
	}

	return nil
}
