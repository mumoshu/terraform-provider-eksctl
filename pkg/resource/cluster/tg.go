package cluster

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func getTGs(d *schema.ResourceData, newID ...string) (map[string]*elbv2.TargetGroup, error) {
	var id string

	if len(newID) > 0 {
		id = newID[0]
	} else {
		id = d.Id()
	}

	svc := elbv2.New(session.New())

	res := map[string]*elbv2.TargetGroup{}

	var targetGroupNames []string

	for _, tgBaseName := range targetGroupNames {

		tgFullName := tgBaseName + "-" + id

		input := &elbv2.DescribeTargetGroupsInput{
			Names: []*string{
				aws.String(tgFullName),
			},
		}

		result, err := svc.DescribeTargetGroups(input)
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				switch aerr.Code() {
				case elbv2.ErrCodeLoadBalancerNotFoundException:
					fmt.Println(elbv2.ErrCodeLoadBalancerNotFoundException, aerr.Error())
				case elbv2.ErrCodeTargetGroupNotFoundException:
					fmt.Println(elbv2.ErrCodeTargetGroupNotFoundException, aerr.Error())
				default:
					fmt.Println(aerr.Error())
				}
			} else {
				// Print the error, cast err to awserr.Error to get the Code and
				// Message from an error.
				fmt.Println(err.Error())
			}
			return nil, err
		}

		tg := result.TargetGroups[0]

		res[tgBaseName] = tg
	}

	return res, nil
}
