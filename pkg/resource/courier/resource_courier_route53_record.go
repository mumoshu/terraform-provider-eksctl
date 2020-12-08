package courier

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/courier"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/tfsdk"
	"github.com/rs/xid"
)

func ResourceRoute53Record() *schema.Resource {
	mSchema := metricSchema()

	return &schema.Resource{
		Create: func(d *schema.ResourceData, meta interface{}) error {
			d.MarkNewResource()

			id := xid.New().String()
			d.SetId(id)

			if err := courier.CreateOrUpdateCourierRoute53Record(&tfsdk.Resource{d}, mSchema); err != nil {
				return fmt.Errorf("updating courier_route53_record: %w", err)
			}
			return nil
		},
		Update: func(d *schema.ResourceData, meta interface{}) error {
			if err := courier.CreateOrUpdateCourierRoute53Record(&tfsdk.Resource{d}, mSchema); err != nil {
				return fmt.Errorf("updating courier_route53_record: %w", err)
			}
			return nil
		},
		CustomizeDiff: func(diff *schema.ResourceDiff, i interface{}) error {
			return nil
		},
		Delete: func(d *schema.ResourceData, meta interface{}) error {
			d.SetId("")

			return nil
		},
		Read: func(d *schema.ResourceData, meta interface{}) error {
			return nil
		},
		Schema: map[string]*schema.Schema{
			"region": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"profile": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"address": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"zone_id": {
				Type:     schema.TypeString,
				Required: true,
			},
			"name": {
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
			"datadog_metric":    MetricsSchema,
			"cloudwatch_metric": MetricsSchema,
			"destination": {
				Type:       schema.TypeList,
				Optional:   true,
				ConfigMode: schema.SchemaConfigModeBlock,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"set_identifier": {
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
