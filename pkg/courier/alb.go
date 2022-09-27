package courier

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk"
	"golang.org/x/xerrors"
)

type CourierALB struct {
	Address          string
	ListenerARN      string
	Priority         int
	ListenerRule     *ListenerRule
	Region           string
	Profile          string
	Destinations     []Destination
	StepWeight       int
	StepInterval     time.Duration
	Metrics          []Metric
	Session          *session.Session
	AssumeRoleConfig *sdk.AssumeRoleConfig
}

type ALB struct {
}

func (a *ALB) Delete(d *CourierALB) error {
	sess := d.Session

	sess.Config.Endpoint = &d.Address

	svc := elbv2.New(sess)

	listenerARN := d.ListenerARN

	o, err := svc.DescribeRules(&elbv2.DescribeRulesInput{
		ListenerArn: aws.String(listenerARN),
	})
	if err != nil {
		return xerrors.Errorf("calling elbv2.DescribeRules: %w", err)
	}

	priority := d.Priority
	priorityStr := strconv.Itoa(priority)

	var rule *elbv2.Rule
	for _, r := range o.Rules {
		if r.Priority != nil && *r.Priority == priorityStr {
			rule = r
		}
	}

	if rule != nil {
		input := &elbv2.DeleteRuleInput{RuleArn: rule.RuleArn}
		if res, err := svc.DeleteRule(input); err != nil {
			var appendix string

			if res != nil {
				appendix = fmt.Sprintf("\nOUTPUT:\n%v", *res)
			}

			log.Printf("Error: deleting rule: %s\nINPUT:\n%v%s", err.Error(), *input, appendix)

			return fmt.Errorf("deleting rule: %w", err)
		}
	}

	return nil
}
