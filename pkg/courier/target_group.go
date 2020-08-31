package courier

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/aws/aws-sdk-go/service/elbv2/elbv2iface"
)

func SetDesiredTGTrafficPercentage(svc elbv2iface.ELBV2API, l ListenerStatus, p int) error {
	if p > 100 {
		return fmt.Errorf("BUG: invalid value for p: got %d, must be less than 100", p)
	}

	if l.DesiredTG == nil {
		return fmt.Errorf("BUG: DesiredTG is nil: %+v", l)
	}

	if l.CurrentTG == nil {
		return fmt.Errorf("BUG: CurrentTG is nil: %+v", l)
	}

	if l.Rule == nil {
		return fmt.Errorf("BUG: Rule is nil: %+v", l)
	}

	_, err := svc.ModifyRule(&elbv2.ModifyRuleInput{
		Actions: []*elbv2.Action{
			{
				ForwardConfig: &elbv2.ForwardActionConfig{
					TargetGroupStickinessConfig: nil,
					TargetGroups: []*elbv2.TargetGroupTuple{
						{
							TargetGroupArn: l.DesiredTG.TargetGroupArn,
							Weight:         aws.Int64(int64(p)),
						}, {
							TargetGroupArn: l.CurrentTG.TargetGroupArn,
							Weight:         aws.Int64(int64(100 - p)),
						},
					},
				},
				Order: aws.Int64(1),
				Type:  aws.String("forward"),
			},
		},
		RuleArn: l.Rule.RuleArn,
	})
	if err != nil {
		return err
	}

	return nil
}
