package tfsdk

import (
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/api"
)

func AWSSessionFromResourceData(d api.Getter, opts ...SchemaOption) *session.Session {
	region, profile := GetAWSRegionAndProfile(d, opts...)

	sess := sdk.NewSession(region, profile)

	assumeRoleConfig := GetAssumeRoleConfig(d, opts...)
	if assumeRoleConfig == nil {
		return sess
	}

	newSess, _, err := sdk.AssumeRole(sess, *assumeRoleConfig)
	if err != nil {
		panic(err)
	}

	return newSess
}
