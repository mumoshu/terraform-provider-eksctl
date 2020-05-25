package main

import (
	"github.com/hashicorp/terraform-plugin-sdk/plugin"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/tfeksctl"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: tfeksctl.Provider})
}
