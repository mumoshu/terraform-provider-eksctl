package provider

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/awsclicompat"
)

type ProviderInstance struct {
	AWSSession *session.Session
}

func providerConfigure() func(*schema.ResourceData) (interface{}, error) {
	return func(d *schema.ResourceData) (interface{}, error) {
		var region string

		if r := d.Get("region"); r != nil {
			if s, ok := r.(string); ok {
				region = s
			}
		}

		s := awsclicompat.NewSession(region)

		return &ProviderInstance{
			AWSSession: s,
		}, nil
	}
}
