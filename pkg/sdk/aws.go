package sdk

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/awsclicompat"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/api"
)

func GetAWSRegionAndProfile(d api.Getter) (string, string) {
	var region string

	if v := d.Get("region"); v != nil {
		region = v.(string)
	}

	var profile string

	if v := d.Get("profile"); v != nil {
		profile = v.(string)
	}

	return region, profile
}

func AWSSessionFromResourceData(d api.Getter) *session.Session {
	region, profile := GetAWSRegionAndProfile(d)

	return awsclicompat.NewSession(region, profile)
}
