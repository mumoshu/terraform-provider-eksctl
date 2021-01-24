WORKSPACE ?= $(shell pwd)
HELMFILE_ROOT ?= ../../../terraform-provider-helmfile
TERRAFORM ?= terraform

.PHONY: build
build: VER=0.0.1
build:
	mkdir -p .terraform/plugins/darwin_amd64
	cd ../..; make clean build
	cp ../../dist/darwin_amd64/terraform-provider-eksctl $(WORKSPACE)/.terraform/plugins/darwin_amd64/
	chmod +x $(WORKSPACE)/.terraform/plugins/darwin_amd64/terraform-provider-eksctl
	cd $(HELMFILE_ROOT); make build
	# For terraform up to v0.12
	cp $(HELMFILE_ROOT)/dist/darwin_amd64/terraform-provider-helmfile $(WORKSPACE)/.terraform/plugins/darwin_amd64/
	chmod +x $(WORKSPACE)/.terraform/plugins/darwin_amd64/terraform-provider-helmfile
	# For tereraform v0.13+
	mkdir -p $(WORKSPACE)/.terraform/plugins/registry.terraform.io/mumoshu/eksctl/$(VER)/darwin_amd64/
	cp ../../dist/darwin_amd64/terraform-provider-eksctl $(WORKSPACE)/.terraform/plugins/registry.terraform.io/mumoshu/eksctl/$(VER)/darwin_amd64/terraform-provider-eksctl_v$(VER)
	chmod +x $(WORKSPACE)/.terraform/plugins/registry.terraform.io/mumoshu/eksctl/$(VER)/darwin_amd64/terraform-provider-eksctl_v$(VER)
	mkdir -p $(WORKSPACE)/.terraform/plugins/registry.terraform.io/mumoshu/helmfile/$(VER)/darwin_amd64/
	cp $(HELMFILE_ROOT)/dist/darwin_amd64/terraform-provider-helmfile $(WORKSPACE)/.terraform/plugins/registry.terraform.io/mumoshu/helmfile/$(VER)/darwin_amd64/terraform-provider-helmfile_v$(VER)
	chmod +x $(WORKSPACE)/.terraform/plugins/registry.terraform.io/mumoshu/helmfile/$(VER)/darwin_amd64/terraform-provider-helmfile_v$(VER)
