package courier

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/google/go-cmp/cmp"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/awsclicompat"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"
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
		desiredRuleConditions := getRuleConditions(lr)

		var conditionsModified bool

		currentConditions := []*elbv2.RuleCondition{}

		if rule.Conditions != nil {
			currentConditions = rule.Conditions
		}

		if d := cmp.Diff(currentConditions, desiredRuleConditions); d != "" {
			conditionsModified = true
		}

		if conditionsModified {
			// ALB doesn't support traffic-weight between different rules.
			// We have no other way than modifying the rule in-place, which means no gradual traffic shiting is done.

			desiredActions := getRuleActions(lr)
			modifyRuleInput := &elbv2.ModifyRuleInput{
				Actions:    desiredActions,
				Conditions: desiredRuleConditions,
				RuleArn:    rule.RuleArn,
			}

			_, err := svc.ModifyRule(modifyRuleInput)
			if err != nil {
				return fmt.Errorf("creating listener rule: %w", err)
			}

			return nil
		}

		// We can gradually shift traffic because Rule.Conditions are unchanged.

		ctx := context.Background()

		var nextTGARN, prevTGARN string

		if destinations[0].Weight > destinations[1].Weight {
			nextTGARN = destinations[0].TargetGroupARN
			prevTGARN = destinations[1].TargetGroupARN
		} else {
			prevTGARN = destinations[0].TargetGroupARN
			nextTGARN = destinations[1].TargetGroupARN
		}

		tgs, err := svc.DescribeTargetGroups(&elbv2.DescribeTargetGroupsInput{
			TargetGroupArns: []*string{
				aws.String(nextTGARN),
				aws.String(prevTGARN),
			},
		})
		if err != nil {
			return err
		}

		var desired, current *elbv2.TargetGroup

		for i := range tgs.TargetGroups {
			tg := *tgs.TargetGroups[i]
			switch *tg.TargetGroupArn {
			case nextTGARN:
				desired = &tg
			case prevTGARN:
				current = &tg
			}
		}

		if desired == nil {
			return xerrors.Errorf("next=desired target group %s not found", nextTGARN)
		}

		if current == nil {
			return xerrors.Errorf("prev=current target group %s not found", prevTGARN)
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
			DesiredTG:      desired,
			CurrentTG:      current,
			DeletedTGs:     nil,
			Metrics:        metrics,
		}

		ctx, cancel := context.WithCancel(ctx)
		e, errctx := errgroup.WithContext(ctx)

		e.Go(func() error {
			defer cancel()
			return DoGradualTrafficShift(errctx, svc, l, 1, CanaryOpts{
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

func getRuleConditions(listenerRule *ListenerRule) []*elbv2.RuleCondition {
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

	return ruleConditions
}

func getRuleActions(listenerRule *ListenerRule) []*elbv2.Action {
	tgs := []*elbv2.TargetGroupTuple{}

	for _, d := range listenerRule.Destinations {
		tgs = append(tgs, &elbv2.TargetGroupTuple{
			TargetGroupArn: aws.String(d.TargetGroupARN),
			Weight:         aws.Int64(int64(d.Weight)),
		})
	}

	ruleActions := []*elbv2.Action{
		{
			ForwardConfig: &elbv2.ForwardActionConfig{
				TargetGroupStickinessConfig: nil,
				TargetGroups:                tgs,
			},
			Type: aws.String("forward"),
		},
	}

	return ruleActions
}

func ruleCreationInput(listenerARN string, listenerRule *ListenerRule) (*elbv2.CreateRuleInput, error) {
	ruleConditions := getRuleConditions(listenerRule)
	ruleActions := getRuleActions(listenerRule)

	createRuleInput := &elbv2.CreateRuleInput{
		Actions:     ruleActions,
		Priority:    aws.Int64(int64(listenerRule.Priority)),
		Conditions:  ruleConditions,
		ListenerArn: aws.String(listenerARN),
	}

	return createRuleInput, nil
}
