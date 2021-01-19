package courier

import (
	"context"
	"errors"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/google/go-cmp/cmp"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"
	"log"
	"strconv"
	"strings"
)

func (a *ALB) Apply(d *CourierALB) error {
	log.SetFlags(log.Lshortfile)

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
	for i := range o.Rules {
		r := o.Rules[i]

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
		log.Printf("Creating new rule for ALB listener %s", listenerARN)

		createRuleInput, err := ruleCreationInput(listenerARN, lr, destinations)
		o, err := svc.CreateRule(createRuleInput)
		if err != nil {
			return fmt.Errorf("creating listener rule: %w", err)
		}

		rule = o.Rules[0]

		log.Printf("Created new rule: %+v", *rule)
	} else {
		log.Printf("Updating existing rule: %+v", *rule)

		desiredRuleConditions := getRuleConditions(lr)

		var conditionsModified bool

		currentConditions := []*elbv2.RuleCondition{}

		if rule.Conditions != nil {
			currentConditions = rule.Conditions
		}

		for i := range rule.Conditions {
			// Otherwise we end up observing changes on Condition.Values even though
			// we can't set both Condition.Values and Condition.*.Values:
			//
			// alb_apply.go:83: Rule conditions has been changed: current (-), desired (+):
			//   []*elbv2.RuleCondition{
			//          &{
			//                  ... // 5 identical fields
			//                  QueryStringConfig: nil,
			//                  SourceIpConfig:    nil,
			// -                Values:            []*string{&"/*"},
			// +                Values:            nil,
			//          },
			//   }
			rule.Conditions[i].Values = nil
		}

		if d := cmp.Diff(currentConditions, desiredRuleConditions); d != "" {
			log.Printf("Rule conditions has been changed: current (-), desired (+):\n%s", d)

			conditionsModified = true
		}

		if conditionsModified {
			log.Printf("Updating rule %s in-place, without traffic shifting", *rule.RuleArn)

			if len(desiredRuleConditions) == 0 {
				return errors.New("ALB does not support rule with no condition(s). Please specify one ore more from `hosts`, `path_patterns`, `methods`, `source_ips` and `headers`")
			}

			// ALB doesn't support traffic-weight between different rules.
			// We have no other way than modifying the rule in-place, which means no gradual traffic shiting is done.

			desiredActions := getRuleActions(destinations)
			modifyRuleInput := &elbv2.ModifyRuleInput{
				Actions:    desiredActions,
				Conditions: desiredRuleConditions,
				RuleArn:    rule.RuleArn,
			}

			_, err := svc.ModifyRule(modifyRuleInput)
			if err != nil {
				return fmt.Errorf("updating listener rule: %w", err)
			}

			return nil
		}

		// We can gradually shift traffic because Rule.Conditions are unchanged.

		log.Printf("Updating rule %s with traffic shifting", *rule.RuleArn)

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
			log.Printf("elbv2.DescribeTargetGroups failed. TargetGroupArns=%v,%v Error=%v", nextTGARN, prevTGARN, err)

			return xerrors.Errorf("calling elbv2.DescribeTargetGroups: %w", err)
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

		log.Printf("Starting to update rule %s, so that the traffic is gradually migrated from %s to %s", *rule.RuleArn, *current.TargetGroupArn, *desired.TargetGroupArn)

		describeListenersResult, err := svc.DescribeListeners(&elbv2.DescribeListenersInput{
			ListenerArns: aws.StringSlice([]string{lr.ListenerARN}),
		})
		if err != nil {
			log.Printf("elbv2.DescribeListeners failed: ListenerArns=%v Error=%v", lr.ListenerARN, err)

			return xerrors.Errorf("calling elbv2.DescribeListeners: %w", err)
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
			return Analyze(errctx, region, profile, d.AssumeRoleConfig, l.Metrics, data)
		})

		if err := e.Wait(); err != nil {
			return xerrors.Errorf("shifting traffic over ALB: %w", err)
		}
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

func getRuleActions(destinations []Destination) []*elbv2.Action {
	tgs := []*elbv2.TargetGroupTuple{}

	for _, d := range destinations {
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

func ruleCreationInput(listenerARN string, listenerRule *ListenerRule, destinations []Destination) (*elbv2.CreateRuleInput, error) {
	ruleConditions := getRuleConditions(listenerRule)
	ruleActions := getRuleActions(destinations)

	createRuleInput := &elbv2.CreateRuleInput{
		Actions:     ruleActions,
		Priority:    aws.Int64(int64(listenerRule.Priority)),
		Conditions:  ruleConditions,
		ListenerArn: aws.String(listenerARN),
	}

	return createRuleInput, nil
}
