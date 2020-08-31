package main

import (
	"github.com/hashicorp/terraform-plugin-sdk/plugin"
	"github.com/mumoshu/terraform-provider-eksctl/pkg/provider"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: provider.Provider})
}
