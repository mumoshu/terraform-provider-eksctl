package courier

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/courier"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/tfsdk"
	"github.com/rs/xid"
	"golang.org/x/xerrors"
	"time"
)

var MetricResourceSchema = map[string]*schema.Schema{
	"aws_region": {
		Type:     schema.TypeString,
		Optional: true,
		Default:  "",
	},
	"aws_profile": {
		Type:     schema.TypeString,
		Optional: true,
		Default:  "",
	},
	"name": {
		Type:     schema.TypeString,
		Required: true,
	},
	"address": {
		Type:     schema.TypeString,
		Optional: true,
		Default:  "",
	},
	"query": {
		Type:     schema.TypeString,
		Required: true,
	},
	"max": {
		Type:     schema.TypeFloat,
		Optional: true,
	},
	"min": {
		Type:     schema.TypeFloat,
		Optional: true,
	},
	"interval": {
		Type:     schema.TypeString,
		Optional: true,
		Default:  "1m",
	},
}

var MetricsSchema = &schema.Schema{
	Type:       schema.TypeList,
	Optional:   true,
	ConfigMode: schema.SchemaConfigModeBlock,
	Elem: &schema.Resource{
		Schema: MetricResourceSchema,
	},
}

func albSchema() *courier.ALBSchema {
	return &courier.ALBSchema{
		Address:                   "address",
		ListenerARN:               "listener_arn",
		Priority:                  "priority",
		Destination:               "destination",
		DestinationTargetGroupARN: "target_group_arn",
		DestinationWeight:         "weight",
		StepWeight:                "step_weight",
		StepInterval:              "step_interval",
		Hosts:                     "hosts",
		PathPatterns:              "path_patterns",
		Methods:                   "methods",
		SourceIPs:                 "source_ips",
		Headers:                   "headers",
		QueryStrings:              "querystrings",
	}
}

func metricSchema() *courier.MetricSchema {
	return &courier.MetricSchema{
		DatadogMetric:    "datadog_metric",
		CloudWatchMetric: "cloudwatch_metric",
		Min:              "min",
		Max:              "max",
		Interval:         "interval",
		Address:          "address",
		Query:            "query",
		AWSProfile:       "aws_profile",
		AWSRegion:        "aws_region",
	}
}

func ResourceALB() *schema.Resource {
	aSchema, mSchema := albSchema(), metricSchema()

	return &schema.Resource{
		Create: func(d *schema.ResourceData, meta interface{}) error {
			d.MarkNewResource()

			id := xid.New().String()
			d.SetId(id)

			if err := courier.CreateOrUpdateCourierALB(&tfsdk.Resource{d}, aSchema, mSchema); err != nil {
				return fmt.Errorf("creating courier_alb: %w", err)
			}
			return nil
		},
		Update: func(d *schema.ResourceData, meta interface{}) error {
			if err := courier.CreateOrUpdateCourierALB(&tfsdk.Resource{d}, aSchema, mSchema); err != nil {
				return fmt.Errorf("updating courier_alb: %w", err)
			}
			return nil
		},
		CustomizeDiff: func(diff *schema.ResourceDiff, i interface{}) error {
			return nil
		},
		Delete: func(d *schema.ResourceData, meta interface{}) error {
			if err := courier.DeleteCourierALB(&tfsdk.Resource{d}, aSchema, mSchema); err != nil {
				return xerrors.Errorf("deleting courier ALB: %w", err)
			}

			d.SetId("")

			return nil
		},
		Read: func(d *schema.ResourceData, meta interface{}) error {
			return nil
		},
		Schema: map[string]*schema.Schema{
			sdk.KeyRegion: {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			sdk.KeyProfile: {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"address": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			sdk.KeyAssumeRole: tfsdk.SchemaAssumeRole(),
			"listener_arn": {
				Type:     schema.TypeString,
				Required: true,
			},
			"step_weight": {
				Type:         schema.TypeInt,
				Required:     true,
				ValidateFunc: validation.IntBetween(1, 100),
			},
			"step_interval": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: ValidateDuration,
			},
			// Listener rule settings
			"priority": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  10,
			},
			"hosts": {
				Type:          schema.TypeSet,
				Optional:      true,
				Set:           schema.HashString,
				Elem:          &schema.Schema{Type: schema.TypeString},
				ConflictsWith: []string{"methods", "path_patterns", "source_ips"},
				Description:   "ALB listener rule condition values for host-header condition, e.g. hosts = [\"example.com\", \"*.example.com\"]",
			},
			"methods": {
				Type:          schema.TypeSet,
				Optional:      true,
				Set:           schema.HashString,
				Elem:          &schema.Schema{Type: schema.TypeString},
				ConflictsWith: []string{"hosts", "path_patterns", "source_ips"},
				Description:   "ALB listener rule condition values for http-request-method condition, e.g. methods = [\"get\"]",
			},
			"path_patterns": {
				Type:          schema.TypeSet,
				Optional:      true,
				Set:           schema.HashString,
				Elem:          &schema.Schema{Type: schema.TypeString},
				ConflictsWith: []string{"hosts", "methods", "source_ips"},
				Description: `
PAthPatternConfig values of ALB listener rule condition "path-pattern" field.

Example:

path_patterns = ["/prefix/*"]

produces:

[
  {
      "Field": "path-pattern",
      "PathPatternConfig": {
          "Values": ["/prefix/*"]
      }
  }
]
`,
			},
			"source_ips": {
				Type:     schema.TypeSet,
				Optional: true,
				Set:      schema.HashString,
				// TF fails with `ValidateFunc is not yet supported on lists or sets.`
				//ValidateFunc:  validation.IPRange(),
				Elem:          &schema.Schema{Type: schema.TypeString},
				ConflictsWith: []string{"hosts", "methods", "path_patterns"},
				Description: `
SourceIpConfig values of ALB listener rule condition "source-ip" field.

Example:

headers = ["MYIPD/CIDR"]

produces:

[
  {
      "Field": "source-ip",
      "SourceIpConfig": {
          "Values": ["MYIP/CIDR"]
      }
  }
]
`,
			},
			"headers": {
				Type: schema.TypeMap,
				Elem: &schema.Schema{
					Type: schema.TypeList,
					Elem: &schema.Schema{Type: schema.TypeString},
				},
				Optional: true,
				Description: `HttpHeaderConfig values of ALB listener rule condition "http-header" field.

Example:

headers = {
 Cookie = "condition=foobar"
}

produces:

[
  {
      "Field": "http-header",
      "HttpHeaderConfig": {
          "HttpHeaderName": "Cookie",
          "Values": ["condition=foobar"]
      }
  }
]
`,
			},
			"querystrings": {
				Type: schema.TypeMap,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Optional: true,
				Description: `QueryStringConfig values of ALB listener rule condition "query-string" field.

Example:

querystrings = {
 foo = "bar"
}

produces:

{
     "Field": "query-string",
     "QueryStringConfig": {
         "Values": [
           {
               "Key": "foo",
               "Value": "bar"
           }
         ]
     }
 }
`,
			},
			"datadog_metric":    MetricsSchema,
			"cloudwatch_metric": MetricsSchema,
			"destination": {
				Type:       schema.TypeList,
				Optional:   true,
				ConfigMode: schema.SchemaConfigModeBlock,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"target_group_arn": {
							Type:     schema.TypeString,
							Required: true,
						},
						"weight": {
							Type:     schema.TypeInt,
							Required: true,
						},
					},
				},
			},
		},
	}
}

func ValidateDuration(v interface{}, k string) (ws []string, errors []error) {
	if _, err := time.ParseDuration(v.(string)); err != nil {
		errors = append(errors, fmt.Errorf("%q: invalid duration", k))
	}
	return
}
