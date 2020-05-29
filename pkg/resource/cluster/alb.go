package cluster

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
)

func getALBs(d *schema.ResourceData) (map[string]*elbv2.LoadBalancer, error) {
	svc := elbv2.New(session.New())

	res := map[string]*elbv2.LoadBalancer{}

	var albNames []string

	for _, albName := range albNames {

		input := &elbv2.DescribeLoadBalancersInput{
			Names: []*string{
				aws.String(albName),
			},
		}

		result, err := svc.DescribeLoadBalancers(input)
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

		alb := result.LoadBalancers[0]

		res[albName] = alb
	}

	return res, nil
}


func attachTG(alb *elbv2.LoadBalancer, newTG *elbv2.TargetGroup) error {
	return nil
}

func shiftTraffic(alb *elbv2.LoadBalancer, newTG *elbv2.TargetGroup, curTG *elbv2.TargetGroup) error {
	return nil
}
