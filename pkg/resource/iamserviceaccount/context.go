package iamserviceaccount

import (
	"github.com/mumoshu/terraform-provider-eksctl/pkg/resource/cluster"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk"
)

func mustContext(a *IAMServiceAccount) *sdk.Context {
	sess, creds := cluster.AWSCredsFromConfig(a.Region, a.Profile, a.AssumeRoleConfig)

	return &sdk.Context{Sess: sess, Creds: creds}
}
