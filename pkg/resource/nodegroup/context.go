package nodegroup

import (
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/sdk/tfsdk"
)

func mustContext(a *schema.ResourceData) *sdk.Context {
	config := tfsdk.ConfigFromResourceData(a)
	sess, creds := sdk.AWSCredsFromConfig(config)

	return &sdk.Context{Sess: sess, Creds: creds}
}
