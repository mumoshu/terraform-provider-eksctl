package provider

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource/cluster"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource/courier"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource/iamserviceaccount"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/tfsdk"
)

// Provider returns a terraform.ResourceProvider.
func Provider() terraform.ResourceProvider {

	// The actual provider
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			tfsdk.KeyAssumeRole: tfsdk.AssumeRoleSchema(),
		},
		ResourcesMap: map[string]*schema.Resource{
			"eksctl_cluster":                cluster.ResourceCluster(),
			"eksctl_iamserviceaccount":      iamserviceaccount.Resource(),
			"eksctl_courier_alb":            courier.ResourceALB(),
			"eksctl_courier_route53_record": courier.ResourceRoute53Record(),
		},
		ConfigureFunc: providerConfigure(),
	}
}
