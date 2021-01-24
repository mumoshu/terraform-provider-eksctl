package tfsdk

import "github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/api"

func GetAWSRegionAndProfile(d api.Getter, opts ...SchemaOption) (string, string) {
	schema := CreateSchema(opts...)

	var region string

	if v := d.Get(schema.KeyAWSRegion); v != nil {
		region = v.(string)
	}

	var profile string

	if v := d.Get(schema.KeyAWSProfile); v != nil {
		profile = v.(string)
	}

	return region, profile
}
