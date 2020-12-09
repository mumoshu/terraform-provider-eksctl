package provider

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/private/protocol/xml/xmlutil"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/elbv2"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/courier/metrics"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

func TestAccCourierALB_create(t *testing.T) {
	resourceName := "eksctl_courier_alb.the_listener"
	_ = acctest.RandString(8)

	appKey := "appKey"
	apiKey := "apiKey"

	expected := 1.11111
	eq := `avg:system.cpu.user{*}by{host}`
	now := time.Now().Unix()
	ddServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		aq := r.URL.Query().Get("query")
		assert.Equal(t, eq, aq)
		assert.Equal(t, appKey, r.Header.Get(metrics.DatadogApplicationKeyHeaderKey))
		assert.Equal(t, apiKey, r.Header.Get(metrics.DatadogAPIKeyHeaderKey))

		from, err := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
		if assert.NoError(t, err) {
			assert.Less(t, from, now)
		}

		to, err := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64)
		if assert.NoError(t, err) {
			assert.GreaterOrEqual(t, to, now)
		}

		json := fmt.Sprintf(`{"series": [{"pointlist": [[1577232000000,29325.102158814265],[1577318400000,56294.46758591842],[1577404800000,%f]]}]}`, expected)
		w.Write([]byte(json))
	}))
	defer ddServer.Close()

	cwServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		aq := r.URL.Query().Get("query")
		assert.Equal(t, eq, aq)

		from, err := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
		if assert.NoError(t, err) {
			assert.Less(t, from, now)
		}

		to, err := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64)
		if assert.NoError(t, err) {
			assert.GreaterOrEqual(t, to, now)
		}

		json := fmt.Sprintf(`{"series": [{"pointlist": [[1577232000000,29325.102158814265],[1577318400000,56294.46758591842],[1577404800000,%f]]}]}`, expected)
		w.Write([]byte(json))
	}))
	defer cwServer.Close()

	albServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("%v", err)
		}

		op := string(body)

		var resBody []byte

		switch op {
		case "Action=DescribeRules&ListenerArn=listener_arn&Version=2015-12-01":
			resBody, err = json.Marshal(&elbv2.DescribeRulesOutput{
				Rules: []*elbv2.Rule{},
			})
		case "Action=CreateRule&Actions.member.1.ForwardConfig.TargetGroups.member.1.TargetGroupArn=prev_arn&Actions.member.1.ForwardConfig.TargetGroups.member.1.Weight=0&Actions.member.1.ForwardConfig.TargetGroups.member.2.TargetGroupArn=next_arn&Actions.member.1.ForwardConfig.TargetGroups.member.2.Weight=100&Actions.member.1.Type=forward&Conditions.member.1.Field=host-header&Conditions.member.1.HostHeaderConfig.Values.member.1=example.com&ListenerArn=listener_arn&Priority=10&Version=2015-12-01":
			params := &elbv2.CreateRuleOutput{
				Rules: []*elbv2.Rule{
					{
						Actions:    nil,
						Conditions: nil,
						IsDefault:  nil,
						Priority:   aws.String("0"),
						RuleArn:    nil,
					},
					{
						Actions:    nil,
						Conditions: nil,
						IsDefault:  nil,
						Priority:   aws.String("100"),
						RuleArn:    nil,
					},
				},
			}
			var buf bytes.Buffer
			err = xmlutil.BuildXML(params, xml.NewEncoder(&buf))
			if err != nil {
				t.Fatalf("%v", err)
			}
			resBody = []byte("<CreateRuleResult>")
			resBody = append(resBody, buf.Bytes()...)
			resBody = append(resBody, []byte("</CreateRuleResult>")...)
		default:
			t.Fatalf("Unexpected operation: %s", op)
		}

		w.WriteHeader(200)
		w.Write(resBody)
	}))
	defer albServer.Close()

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckCourierALBListenerDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCourierALBConfig_basic(`"`+ddServer.URL+`"`, `"`+cwServer.URL+`"`, `"`+albServer.URL+`"`, `"prev_arn"`, `"next_arn"`, `"listener_arn"`),

				Check: resource.ComposeTestCheckFunc(
					//resource.TestCheckResourceAttr(resourceName, "selector.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "destination.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "destination.0.target_group_arn", "prev_arn"),
					resource.TestCheckResourceAttr(resourceName, "destination.0.weight", "0"),
					resource.TestCheckResourceAttr(resourceName, "datadog_metric.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "cloudwatch_metric.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "destination.1.target_group_arn", "next_arn"),
					resource.TestCheckResourceAttr(resourceName, "destination.1.weight", "100"),
					//resource.TestCheckResourceAttr(resourceName, "diff_output", wantedHelmfileDiffOutputForReleaseID(releaseID)),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
		},
	})
}

func TestAccCourierALB_update(t *testing.T) {
	resourceName := "eksctl_courier_alb.the_listener"
	_ = acctest.RandString(8)

	appKey := "appKey"
	apiKey := "apiKey"

	os.Setenv("DATADOG_API_KEY", apiKey)
	os.Setenv("DATADOG_APPLICATION_KEY", appKey)

	expected := 1.11111
	eq := `avg:system.cpu.user{*}by{host}`
	now := time.Now().Unix()
	ddServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		aq := r.URL.Query().Get("query")
		assert.Equal(t, eq, aq)
		assert.Equal(t, appKey, r.Header.Get(metrics.DatadogApplicationKeyHeaderKey))
		assert.Equal(t, apiKey, r.Header.Get(metrics.DatadogAPIKeyHeaderKey))

		from, err := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
		if assert.NoError(t, err) {
			assert.Less(t, from, now)
		}

		to, err := strconv.ParseInt(r.URL.Query().Get("to"), 10, 64)
		if assert.NoError(t, err) {
			assert.GreaterOrEqual(t, to, now)
		}

		json := fmt.Sprintf(`{"series": [{"pointlist": [[1577232000000,29325.102158814265],[1577318400000,56294.46758591842],[1577404800000,%f]]}]}`, expected)
		w.Write([]byte(json))
	}))
	defer ddServer.Close()

	cwServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("%v", err)
		}

		q := string(body)

		var resBody []byte

		if strings.HasPrefix(q, "Action=GetMetricData&") {
			params := &cloudwatch.GetMetricDataOutput{
				MetricDataResults: []*cloudwatch.MetricDataResult{
					{
						Id:         nil,
						Label:      nil,
						Messages:   nil,
						StatusCode: nil,
						Timestamps: nil,
						Values:     []*float64{aws.Float64(10)},
					},
				},
			}
			var buf bytes.Buffer
			err = xmlutil.BuildXML(params, xml.NewEncoder(&buf))
			if err != nil {
				t.Fatalf("%v", err)
			}
			resBody = []byte("<GetMetricDataResult>")
			resBody = append(resBody, buf.Bytes()...)
			resBody = append(resBody, []byte("</GetMetricDataResult>")...)
		} else {
			t.Fatalf("Unexpected cloudwatch query: %s", q)
		}

		w.Write(resBody)
	}))
	defer cwServer.Close()

	albServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("%v", err)
		}

		op := string(body)

		var resBody []byte

		switch op {
		case "Action=DescribeRules&ListenerArn=listener_arn&Version=2015-12-01":
			params := &elbv2.DescribeRulesOutput{
				Rules: []*elbv2.Rule{
					{
						RuleArn: aws.String("rule_arn"),
						Actions: []*elbv2.Action{
							{
								ForwardConfig: &elbv2.ForwardActionConfig{
									TargetGroupStickinessConfig: nil,
									TargetGroups: []*elbv2.TargetGroupTuple{
										{
											TargetGroupArn: aws.String("prev_arn"),
											Weight:         aws.Int64(100),
										},
									},
								},
							},
						},
						Priority: aws.String("10"),
					},
				},
			}
			var buf bytes.Buffer
			err = xmlutil.BuildXML(params, xml.NewEncoder(&buf))
			if err != nil {
				t.Fatalf("%v", err)
			}
			resBody = []byte("<DescribeRulesResult>")
			resBody = append(resBody, buf.Bytes()...)
			resBody = append(resBody, []byte("</DescribeRulesResult>")...)
		case "Action=DescribeTargetGroups&TargetGroupArns.member.1=next_arn&TargetGroupArns.member.2=prev_arn&Version=2015-12-01":
			params := &elbv2.DescribeTargetGroupsOutput{
				TargetGroups: []*elbv2.TargetGroup{
					{
						TargetGroupArn:  aws.String("next_arn"),
						TargetGroupName: aws.String("Next"),
					},
					{
						TargetGroupArn:  aws.String("prev_arn"),
						TargetGroupName: aws.String("Prev"),
					},
				},
			}
			var buf bytes.Buffer
			err = xmlutil.BuildXML(params, xml.NewEncoder(&buf))
			if err != nil {
				t.Fatalf("%v", err)
			}
			resBody = []byte("<DescribeTargetGroupsResult>")
			resBody = append(resBody, buf.Bytes()...)
			resBody = append(resBody, []byte("</DescribeTargetGroupsResult>")...)
		case "Action=DescribeListeners&ListenerArns.member.1=listener_arn&Version=2015-12-01":
			params := &elbv2.DescribeListenersOutput{
				Listeners: []*elbv2.Listener{
					{
						ListenerArn: aws.String("listener_arn"),
					},
				},
			}
			var buf bytes.Buffer
			err = xmlutil.BuildXML(params, xml.NewEncoder(&buf))
			if err != nil {
				t.Fatalf("%v", err)
			}
			resBody = []byte("<DescribeListenersResult>")
			resBody = append(resBody, buf.Bytes()...)
			resBody = append(resBody, []byte("</DescribeListenersResult>")...)
		case
			"Action=ModifyRule&Actions.member.1.ForwardConfig.TargetGroups.member.1.TargetGroupArn=prev_arn&Actions.member.1.ForwardConfig.TargetGroups.member.1.Weight=0&Actions.member.1.ForwardConfig.TargetGroups.member.2.TargetGroupArn=next_arn&Actions.member.1.ForwardConfig.TargetGroups.member.2.Weight=100&Actions.member.1.Type=forward&Conditions.member.1.Field=host-header&Conditions.member.1.HostHeaderConfig.Values.member.1=example.com&RuleArn=rule_arn&Version=2015-12-01",
			"Action=ModifyRule&Actions.member.1.ForwardConfig.TargetGroups.member.1.TargetGroupArn=prev_arn&Actions.member.1.ForwardConfig.TargetGroups.member.1.Weight=1&Actions.member.1.ForwardConfig.TargetGroups.member.2.TargetGroupArn=next_arn&Actions.member.1.ForwardConfig.TargetGroups.member.2.Weight=99&Actions.member.1.Type=forward&Conditions.member.1.Field=host-header&Conditions.member.1.HostHeaderConfig.Values.member.1=example.com&RuleArn=rule_arn&Version=2015-12-01",
			"Action=ModifyRule&Actions.member.1.ForwardConfig.TargetGroups.member.1.TargetGroupArn=prev_arn&Actions.member.1.ForwardConfig.TargetGroups.member.1.Weight=51&Actions.member.1.ForwardConfig.TargetGroups.member.2.TargetGroupArn=next_arn&Actions.member.1.ForwardConfig.TargetGroups.member.2.Weight=49&Actions.member.1.Type=forward&Conditions.member.1.Field=host-header&Conditions.member.1.HostHeaderConfig.Values.member.1=example.com&RuleArn=rule_arn&Version=2015-12-01",
			"Action=ModifyRule&Actions.member.1.ForwardConfig.TargetGroups.member.1.TargetGroupArn=prev_arn&Actions.member.1.ForwardConfig.TargetGroups.member.1.Weight=100&Actions.member.1.ForwardConfig.TargetGroups.member.2.TargetGroupArn=next_arn&Actions.member.1.ForwardConfig.TargetGroups.member.2.Weight=0&Actions.member.1.Type=forward&Conditions.member.1.Field=host-header&Conditions.member.1.HostHeaderConfig.Values.member.1=example.com&RuleArn=rule_arn&Version=2015-12-01":

			params := &elbv2.CreateRuleOutput{
				Rules: []*elbv2.Rule{
					{
						Actions: []*elbv2.Action{
							{
								ForwardConfig: &elbv2.ForwardActionConfig{
									TargetGroupStickinessConfig: nil,
									TargetGroups: []*elbv2.TargetGroupTuple{
										{
											TargetGroupArn: aws.String("prev_arn"),
											Weight:         aws.Int64(0),
										},
										{
											TargetGroupArn: aws.String("next_arn"),
											Weight:         aws.Int64(100),
										},
									},
								},
							},
						},
						Priority: aws.String("10"),
					},
				},
			}
			var buf bytes.Buffer
			err = xmlutil.BuildXML(params, xml.NewEncoder(&buf))
			if err != nil {
				t.Fatalf("%v", err)
			}
			resBody = []byte("<CreateRuleResult>")
			resBody = append(resBody, buf.Bytes()...)
			resBody = append(resBody, []byte("</CreateRuleResult>")...)

		case "Action=DeleteRule&RuleArn=rule_arn&Version=2015-12-01":

		default:
			t.Fatalf("Unexpected operation: %s", op)
		}

		w.WriteHeader(200)
		w.Write(resBody)
	}))
	defer albServer.Close()

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckCourierALBListenerDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCourierALBConfig_basic(`"`+ddServer.URL+`"`, `"`+cwServer.URL+`"`, `"`+albServer.URL+`"`, `"prev_arn"`, `"next_arn"`, `"listener_arn"`),

				Check: resource.ComposeTestCheckFunc(
					//resource.TestCheckResourceAttr(resourceName, "selector.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "destination.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "destination.0.target_group_arn", "prev_arn"),
					resource.TestCheckResourceAttr(resourceName, "destination.0.weight", "0"),
					resource.TestCheckResourceAttr(resourceName, "datadog_metric.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "cloudwatch_metric.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "destination.1.target_group_arn", "next_arn"),
					resource.TestCheckResourceAttr(resourceName, "destination.1.weight", "100"),
					//resource.TestCheckResourceAttr(resourceName, "diff_output", wantedHelmfileDiffOutputForReleaseID(releaseID)),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
		},
	})
}

func testAccCheckCourierALBListenerDestroy(s *terraform.State) error {
	_ = testAccProvider.Meta().(*ProviderInstance)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "eksctl_courier_alb" {
			continue
		}
		//
		//helmfileYaml := fmt.Sprintf("helmfile-%s.yaml", rs.Primary.ID)
		//
		//cmd := exec.Command("helmfile", "-f", helmfileYaml, "status")
		//if out, err := cmd.CombinedOutput(); err == nil {
		//	return fmt.Errorf("verifying helmfile status: releases still exist for %s", helmfileYaml)
		//} else if !strings.Contains(string(out), "Error: release: not found") {
		//	return fmt.Errorf("verifying helmfile status: unexpected error: %v:\n\nCOMBINED OUTPUT:\n%s", err, string(out))
		//}
	}
	return nil
}

func testAccCourierALBConfig_basic(ddEndpoint, cwEndpoint, albEndpoint, prevTGARN, nextTGARN, listenerARN string) string {
	r := strings.NewReplacer(
		"vars.prev_target_group_arn", prevTGARN,
		"vars.next_target_group_arn", nextTGARN,
		"vars.dd_endpoint", ddEndpoint,
		"vars.cw_endpoint", cwEndpoint,
		"aws_alb_listener.arn", listenerARN,
		"vars.alb_endpoint", albEndpoint,
	)
	return r.Replace(`
resource "eksctl_courier_alb" "the_listener" {
  address = vars.alb_endpoint

  listener_arn = aws_alb_listener.arn

  step_weight = 50
  step_interval = "1s"

  hosts = ["example.com"]
  
  destination {
    target_group_arn = vars.prev_target_group_arn

    weight = 0 
  }

  destination {
    target_group_arn = vars.next_target_group_arn
    weight = 100
  }

  cloudwatch_metric {
    name = "http_errors_cw"

    # it will query from <now - 60 sec> to now, every 60 sec
    interval = "1m"

    max = 50

    query = <<EOQ
[
    {
        "Id": "e1",
        "Expression": "m1 / m2",
        "Label": "ErrorRate"
    },
    {
        "Id": "m1",
        "MetricStat": {
            "Metric": {
                "Namespace": "MyApplication",
                "MetricName": "Errors",
                "Dimensions": [
                    {
                        "Name": "FunctionName",
                        "Value": "MyFunc"
                    }
                ]
            },
            "Period": 300,
            "Stat": "Sum",
            "Unit": "Count"
        },
        "ReturnData": false
    },
    {
        "Id": "m2",
        "MetricStat": {
            "Metric": {
                "Namespace": "MyApplication",
                "MetricName": "Invocations",
                "Dimensions": [
                    {
                        "Name": "FunctionName",
                        "Value": "MyFunc"
                    }
                ]
            },
            "Period": 300,
            "Stat": "Sum",
            "Unit": "Count"
        },
        "ReturnData": false
    }
]
EOQ

    address = vars.cw_endpoint
  }

  datadog_metric {
    name = "http_errors_dd"

    # it will query from <now - 60 sec> to now, every 60 sec
    interval = "1m"

    max = 50

    query = "avg:system.cpu.user{*}by{host}"

    address = vars.dd_endpoint
  }
}
`)
}
