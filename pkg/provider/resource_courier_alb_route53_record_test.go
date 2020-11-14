package provider

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/private/protocol/xml/xmlutil"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
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
)

func TestAccCourierRoute53Record_create(t *testing.T) {
	resourceName := "eksctl_courier_route53_record.the_record"
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

	cwQuery := `
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
`

	cwServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req cloudwatch.GetMetricDataInput
		if err := xmlutil.UnmarshalXML(&req, xml.NewDecoder(r.Body), "GetMetricDataInputRequest"); err != nil {
			t.Fatalf("Unexpected error while unmarshalling XML: %v", err)
		}

		var expected []*cloudwatch.MetricDataQuery
		if err := json.Unmarshal([]byte(cwQuery), &expected); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if diff := cmp.Diff(expected, req.MetricDataQueries); diff != "" {
			t.Fatalf("Unexpected diff: %s", diff)
		}

		params := &cloudwatch.GetMetricDataOutput{
			Messages: nil,
			MetricDataResults: []*cloudwatch.MetricDataResult{
				{
					Id:         nil,
					Label:      nil,
					Messages:   nil,
					StatusCode: nil,
					Timestamps: nil,
					Values: []*float64{
						aws.Float64(10),
					},
				},
			},
			NextToken: nil,
		}
		var buf bytes.Buffer
		err := xmlutil.BuildXML(params, xml.NewEncoder(&buf))
		if err != nil {
			t.Fatalf("%v", err)
		}
		resBody := []byte("<GetMetricDataResult>")
		resBody = append(resBody, buf.Bytes()...)
		resBody = append(resBody, []byte("</GetMetricDataResult>")...)

		w.Write(resBody)
	}))
	defer cwServer.Close()

	weights := map[string]int64{
		"next_id": 0,
		"prev_id": 100,
	}

	r53Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("%v", err)
		}

		op := string(body)

		var resBody []byte

		switch r.RequestURI {
		case "/2013-04-01/hostedzone/zone_id":
			// courier_route53_record checks the existence of hosted zone for fail-fast
		case "/2013-04-01/hostedzone/zone_id/rrset?name=record_name":
			params := &route53.ListResourceRecordSetsOutput{
				// TODO This is a workaround for progressived's bug that treats IsTruncated's meaning as it's oppposite
				// Set this `aws.Bool(false)` once we fix that.
				IsTruncated: aws.Bool(false),
				ResourceRecordSets: []*route53.ResourceRecordSet{
					{
						Name:          aws.String("record_name"),
						SetIdentifier: aws.String("next_id"),
						Weight:        aws.Int64(weights["next_id"]),
						Type:          aws.String("A"),
					},
					{
						Name:          aws.String("record_name"),
						SetIdentifier: aws.String("prev_id"),
						Weight:        aws.Int64(weights["prev_id"]),
						Type:          aws.String("A"),
					},
				},
			}
			var buf bytes.Buffer
			err = xmlutil.BuildXML(params, xml.NewEncoder(&buf))
			if err != nil {
				t.Fatalf("%v", err)
			}
			resBody = []byte("<ListResourceRecordSetsResult>")
			resBody = append(resBody, buf.Bytes()...)
			resBody = append(resBody, []byte("</ListResourceRecordSetsResult>")...)
		case "/2013-04-01/hostedzone/zone_id/rrset":
			var req route53.ChangeResourceRecordSetsInput
			if err := xmlutil.UnmarshalXML(&req, xml.NewDecoder(strings.NewReader(op)), "ChangeResourceRecordSetsRequest"); err != nil {
				t.Fatalf("Unexpected error while unmarshalling XML: %v", err)
			}
			for _, c := range req.ChangeBatch.Changes {
				weights[*c.ResourceRecordSet.SetIdentifier] = *c.ResourceRecordSet.Weight
			}

			params := &route53.ChangeResourceRecordSetsOutput{
				ChangeInfo: &route53.ChangeInfo{
					Comment:     nil,
					Id:          nil,
					Status:      nil,
					SubmittedAt: nil,
				},
			}
			var buf bytes.Buffer
			err = xmlutil.BuildXML(params, xml.NewEncoder(&buf))
			if err != nil {
				t.Fatalf("%v", err)
			}
			resBody = []byte("<ChangeResourceRecordSetsResult>")
			resBody = append(resBody, buf.Bytes()...)
			resBody = append(resBody, []byte("</ChangeResourceRecordSetsResult>")...)
		default:
			t.Fatalf("Unexpected operation: uri=%s, body=%s", r.RequestURI, op)
		}

		w.WriteHeader(200)
		w.Write(resBody)
	}))
	defer r53Server.Close()

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckCourierRoute53RecordDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCourierRoute53RecordConfig_basic(`"`+ddServer.URL+`"`, `"`+cwServer.URL+`"`, `"`+r53Server.URL+`"`, `"prev_id"`, `"next_id"`, `"zone_id"`, `"record_name"`),

				Check: resource.ComposeTestCheckFunc(
					//resource.TestCheckResourceAttr(resourceName, "selector.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "destination.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "destination.0.set_identifier", "prev_id"),
					resource.TestCheckResourceAttr(resourceName, "destination.0.weight", "0"),
					resource.TestCheckResourceAttr(resourceName, "datadog_metric.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "cloudwatch_metric.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "destination.1.set_identifier", "next_id"),
					resource.TestCheckResourceAttr(resourceName, "destination.1.weight", "100"),
					//resource.TestCheckResourceAttr(resourceName, "diff_output", wantedHelmfileDiffOutputForReleaseID(releaseID)),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
		},
	})
}

func testAccCheckCourierRoute53RecordDestroy(s *terraform.State) error {
	_ = testAccProvider.Meta().(*ProviderInstance)

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "eksctl_courier_route53_record" {
			continue
		}
	}
	return nil
}

func testAccCourierRoute53RecordConfig_basic(ddEndpoint, cwEndpoint, r53Endpoint, prevId, nextId, zoneId, recordName string) string {
	r := strings.NewReplacer(
		"var.prev_set_identifier", prevId,
		"var.next_set_identifier", nextId,
		"var.dd_endpoint", ddEndpoint,
		"var.cw_endpoint", cwEndpoint,
		"var.zone_id", zoneId,
		"var.record_name", recordName,
		"var.route53_endpoint", r53Endpoint,
	)
	return r.Replace(`
resource "eksctl_courier_route53_record" "the_record" {
  address = var.route53_endpoint

  zone_id = var.zone_id
  name = var.record_name

  step_weight = 50
  step_interval = "1s"
  
  destination {
    set_identifier = var.prev_set_identifier

    weight = 0 
  }

  destination {
    set_identifier = var.next_set_identifier
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

    address = var.cw_endpoint
  }

  datadog_metric {
    name = "http_errors_dd"

    # it will query from <now - 60 sec> to now, every 60 sec
    interval = "1m"

    max = 50

    query = "avg:system.cpu.user{*}by{host}"

    address = var.dd_endpoint
  }
}
`)
}
