package iamserviceaccount

import (
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk"
)

func mustContext(a *IAMServiceAccount) *sdk.Context {
	sess, creds := sdk.AWSCredsFromValues(a.Region, a.Profile, a.AssumeRoleConfig)

	return &sdk.Context{Sess: sess, Creds: creds}
}
