package tfsdk

import "github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/api"

func GetAWSRegionAndProfile(d api.Getter) (string, string) {
	var region string

	if v := d.Get(KeyRegion); v != nil {
		region = v.(string)
	}

	var profile string

	if v := d.Get(KeyProfile); v != nil {
		profile = v.(string)
	}

	return region, profile
}
