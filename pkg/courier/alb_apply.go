package courier

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/awsclicompat"
	"golang.org/x/sync/errgroup"
	"strconv"
	"strings"
)

func (a *ALB) Apply(d *CourierALB) error {
	sess := awsclicompat.NewSession(d.Region, d.Profile)

	sess.Config.Endpoint = &d.Address

	svc := elbv2.New(sess)

	listenerARN := d.ListenerARN

	o, err := svc.DescribeRules(&elbv2.DescribeRulesInput{
		ListenerArn: aws.String(listenerARN),
	})
	if err != nil {
		return err
	}

	priority := d.Priority
	priorityStr := strconv.Itoa(priority)

	var rule *elbv2.Rule
	for _, r := range o.Rules {
		if r.Priority != nil && *r.Priority == priorityStr {
			rule = r
		}
	}

	lr := d.ListenerRule

	metrics := d.Metrics

	destinations := d.Destinations

	stepInterval := d.StepInterval

	stepWeight := d.StepWeight

	if rule == nil {
		lr.Destinations = destinations

		createRuleInput, err := ruleCreationInput(listenerARN, lr)
		o, err := svc.CreateRule(createRuleInput)
		if err != nil {
			return fmt.Errorf("creating listener rule: %w", err)
		}

		rule = o.Rules[0]
	} else {
		ctx := context.Background()

		var nextTGARN, prevTGARN string

		if destinations[0].Weight > destinations[1].Weight {
			nextTGARN = destinations[0].TargetGroupARN
			prevTGARN = destinations[1].TargetGroupARN
		} else {
			prevTGARN = destinations[0].TargetGroupARN
			nextTGARN = destinations[1].TargetGroupARN
		}

		r1, err := svc.DescribeTargetGroups(&elbv2.DescribeTargetGroupsInput{
			TargetGroupArns: []*string{
				aws.String(nextTGARN),
			},
		})
		if err != nil {
			return err
		}

		r2, err := svc.DescribeTargetGroups(&elbv2.DescribeTargetGroupsInput{
			TargetGroupArns: []*string{
				aws.String(prevTGARN),
			},
		})
		if err != nil {
			return err
		}

		describeListenersResult, err := svc.DescribeListeners(&elbv2.DescribeListenersInput{
			ListenerArns: aws.StringSlice([]string{lr.ListenerARN}),
		})
		if err != nil {
			return err
		}

		l := ListenerStatus{
			Listener:       describeListenersResult.Listeners[0],
			Rule:           rule,
			ALBAttachments: nil,
			DesiredTG:      r1.TargetGroups[0],
			CurrentTG:      r2.TargetGroups[0],
			DeletedTGs:     nil,
			Metrics:        metrics,
		}

		ctx, cancel := context.WithCancel(ctx)
		e, errctx := errgroup.WithContext(ctx)

		e.Go(func() error {
			defer cancel()
			return DoGradualTrafficShift(errctx, svc, l, CanaryOpts{
				CanaryAdvancementInterval: stepInterval,
				CanaryAdvancementStep:     stepWeight,
				Region:                    "",
				ClusterName:               "",
			})
		})

		data := ListerStatusToTemplateData(l)

		region, profile := d.Region, d.Profile

		e.Go(func() error {
			return Analyze(errctx, region, profile, l.Metrics, data)
		})

		if err := e.Wait(); err != nil {
			return err
		}
	}

	if err != nil {
		return err
	}
	return nil
}

func ruleCreationInput(listenerARN string, listenerRule *ListenerRule) (*elbv2.CreateRuleInput, error) {
	// Create rule and set it to l.Rule
	ruleConditions := []*elbv2.RuleCondition{
		//	{
		//		Field:                   nil,
		//		HostHeaderConfig:        nil,
		//		HttpHeaderConfig:        nil,
		//		HttpRequestMethodConfig: nil,
		//		PathPatternConfig:       nil,
		//		QueryStringConfig:       nil,
		//		SourceIpConfig:          nil,
		//		Values:                  nil,
		//	}
	}

	// See this for how rule conditions should be composed:
	// https://cloudaffaire.com/aws-application-load-balancer-listener-rules-and-advance-routing-options
	// (I found it much readable and helpful than the official reference doc

	if len(listenerRule.Hosts) > 0 {
		ruleConditions = append(ruleConditions, &elbv2.RuleCondition{
			Field: aws.String("host-header"),
			HostHeaderConfig: &elbv2.HostHeaderConditionConfig{
				Values: aws.StringSlice(listenerRule.Hosts),
			},
		})
	}

	if len(listenerRule.PathPatterns) > 0 {
		ruleConditions = append(ruleConditions, &elbv2.RuleCondition{
			Field: aws.String("path-pattern"),
			PathPatternConfig: &elbv2.PathPatternConditionConfig{
				Values: aws.StringSlice(listenerRule.PathPatterns),
			},
		})
	}

	if len(listenerRule.Methods) > 0 {
		methods := make([]string, len(listenerRule.Methods))

		for i, m := range listenerRule.Methods {
			methods[i] = strings.ToUpper(m)
		}

		ruleConditions = append(ruleConditions, &elbv2.RuleCondition{
			Field: aws.String("http-request-method"),
			HttpRequestMethodConfig: &elbv2.HttpRequestMethodConditionConfig{
				Values: aws.StringSlice(methods),
			},
		})
	}

	if len(listenerRule.SourceIPs) > 0 {
		ruleConditions = append(ruleConditions, &elbv2.RuleCondition{
			Field: aws.String("source-ip"),
			SourceIpConfig: &elbv2.SourceIpConditionConfig{
				Values: aws.StringSlice(listenerRule.SourceIPs),
			},
		})
	}

	if len(listenerRule.Headers) > 0 {
		for name, values := range listenerRule.Headers {
			ruleConditions = append(ruleConditions, &elbv2.RuleCondition{
				Field: aws.String("http-header"),
				HttpHeaderConfig: &elbv2.HttpHeaderConditionConfig{
					HttpHeaderName: aws.String(name),
					Values:         aws.StringSlice(values),
				},
			})
		}
	}

	if len(listenerRule.QueryStrings) > 0 {
		var vs []*elbv2.QueryStringKeyValuePair

		for k, v := range listenerRule.QueryStrings {
			vs = append(vs, &elbv2.QueryStringKeyValuePair{
				Key:   aws.String(k),
				Value: aws.String(v),
			})
		}
		ruleConditions = append(ruleConditions, &elbv2.RuleCondition{
			Field: aws.String("query-string"),
			QueryStringConfig: &elbv2.QueryStringConditionConfig{
				Values: vs,
			},
		})
	}

	tgs := []*elbv2.TargetGroupTuple{}

	for _, d := range listenerRule.Destinations {
		tgs = append(tgs, &elbv2.TargetGroupTuple{
			TargetGroupArn: aws.String(d.TargetGroupARN),
			Weight:         aws.Int64(int64(d.Weight)),
		})
	}

	createRuleInput := &elbv2.CreateRuleInput{
		Actions: []*elbv2.Action{
			{
				ForwardConfig: &elbv2.ForwardActionConfig{
					TargetGroupStickinessConfig: nil,
					TargetGroups:                tgs,
				},
				Type: aws.String("forward"),
			},
		},
		Priority:    aws.Int64(int64(listenerRule.Priority)),
		Conditions:  ruleConditions,
		ListenerArn: aws.String(listenerARN),
	}

	return createRuleInput, nil
}
