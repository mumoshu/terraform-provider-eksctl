package provider

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/awsclicompat"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/courier"
	"golang.org/x/sync/errgroup"
	"strconv"
	"strings"
	"time"
)

func createOrUpdateCourierALB(d *schema.ResourceData) error {
	var region string
	if v := d.Get("region"); v != nil {
		region = v.(string)
	}

	sess := awsclicompat.NewSession(region)

	if v := d.Get("address"); v != nil {
		sess.Config.Endpoint = aws.String(v.(string))
	}

	svc := elbv2.New(sess)

	listenerARN := d.Get("listener_arn").(string)

	o, err := svc.DescribeRules(&elbv2.DescribeRulesInput{
		ListenerArn: aws.String(listenerARN),
	})
	if err != nil {
		return err
	}

	priority := d.Get("priority").(int)
	priorityStr := strconv.Itoa(priority)

	var rule *elbv2.Rule
	for _, r := range o.Rules {
		if r.Priority != nil && *r.Priority == priorityStr {
			rule = r
		}
	}

	lr, err := courier.ReadListenerRule(d)
	if err != nil {
		return err
	}

	metrics, err := readMetrics(d)
	if err != nil {
		return err
	}

	var destinations []courier.Destination

	if v := d.Get("destination"); v != nil {
		for _, arrayItem := range v.([]interface{}) {
			m := arrayItem.(map[string]interface{})
			tgARN := m["target_group_arn"].(string)
			weight := m["weight"].(int)

			d := courier.Destination{
				TargetGroupARN: tgARN,
				Weight:         weight,
			}

			destinations = append(destinations, d)
		}
	}

	stepInterval := 1 * time.Second
	if v := d.Get("step_interval"); v != nil {
		d, err := time.ParseDuration(v.(string))
		if err != nil {
			return fmt.Errorf("error parsing step_interval %v: %w", v, err)
		}

		stepInterval = d
	}

	stepWeight := 50
	if v := d.Get("step_weight"); v != nil {
		stepWeight = v.(int)
	}

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

		l := courier.ListenerStatus{
			Listener:       describeListenersResult.Listeners[0],
			Rule:           rule,
			ALBAttachments: nil,
			DesiredTG:      r1.TargetGroups[0],
			CurrentTG:      r1.TargetGroups[1],
			DeletedTGs:     nil,
			Metrics:        metrics,
		}

		ctx, cancel := context.WithCancel(ctx)
		e, errctx := errgroup.WithContext(ctx)

		e.Go(func() error {
			defer cancel()
			return courier.DoGradualTrafficShift(errctx, svc, l, courier.CanaryOpts{
				CanaryAdvancementInterval: stepInterval,
				CanaryAdvancementStep:     stepWeight,
				Region:                    "",
				ClusterName:               "",
			})
		})

		data := courier.ListerStatusToTemplateData(l)

		e.Go(func() error {
			return courier.Analyze(errctx, region, l.Metrics, data)
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

func ruleCreationInput(listenerARN string, listenerRule *courier.ListenerRule) (*elbv2.CreateRuleInput, error) {
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
