WORKSPACE ?= $(shell pwd)
HELMFILE_ROOT ?= ../../../terraform-provider-helmfile
KUBECTL_ROOT ?= ../../../terraform-provider-kubectl
TERRAFORM ?= terraform

.PHONY: build
build: VER=0.0.1
build:
	mkdir -p .terraform/plugins/darwin_amd64
	cd ../..; make clean build
	cd $(HELMFILE_ROOT); make build
	cd $(KUBECTL_ROOT); make build
	# For terraform up to v0.12
	#
	# eksctl
	cp ../../dist/darwin_amd64/terraform-provider-eksctl $(WORKSPACE)/.terraform/plugins/darwin_amd64/
	chmod +x $(WORKSPACE)/.terraform/plugins/darwin_amd64/terraform-provider-eksctl
	# helmfile
	cp $(HELMFILE_ROOT)/dist/darwin_amd64/terraform-provider-helmfile $(WORKSPACE)/.terraform/plugins/darwin_amd64/
	chmod +x $(WORKSPACE)/.terraform/plugins/darwin_amd64/terraform-provider-helmfile
	# kubectl
	cp $(KUBECTL_ROOT)/dist/darwin_amd64/terraform-provider-kubectl $(WORKSPACE)/.terraform/plugins/darwin_amd64/
	chmod +x $(WORKSPACE)/.terraform/plugins/darwin_amd64/terraform-provider-kubectl
	# For tereraform v0.13+
	#
	# eksctl
	mkdir -p $(WORKSPACE)/.terraform/plugins/registry.terraform.io/mumoshu/eksctl/$(VER)/darwin_amd64/
	cp ../../dist/darwin_amd64/terraform-provider-eksctl $(WORKSPACE)/.terraform/plugins/registry.terraform.io/mumoshu/eksctl/$(VER)/darwin_amd64/terraform-provider-eksctl_v$(VER)
	chmod +x $(WORKSPACE)/.terraform/plugins/registry.terraform.io/mumoshu/eksctl/$(VER)/darwin_amd64/terraform-provider-eksctl_v$(VER)
	# helmfile
	mkdir -p $(WORKSPACE)/.terraform/plugins/registry.terraform.io/mumoshu/helmfile/$(VER)/darwin_amd64/
	cp $(HELMFILE_ROOT)/dist/darwin_amd64/terraform-provider-helmfile $(WORKSPACE)/.terraform/plugins/registry.terraform.io/mumoshu/helmfile/$(VER)/darwin_amd64/terraform-provider-helmfile_v$(VER)
	chmod +x $(WORKSPACE)/.terraform/plugins/registry.terraform.io/mumoshu/helmfile/$(VER)/darwin_amd64/terraform-provider-helmfile_v$(VER)
	# kubectl
	mkdir -p $(WORKSPACE)/.terraform/plugins/registry.terraform.io/mumoshu/kubectl/$(VER)/darwin_amd64/
	cp $(KUBECTL_ROOT)/dist/darwin_amd64/terraform-provider-kubectl $(WORKSPACE)/.terraform/plugins/registry.terraform.io/mumoshu/kubectl/$(VER)/darwin_amd64/terraform-provider-kubectl_v$(VER)
	chmod +x $(WORKSPACE)/.terraform/plugins/registry.terraform.io/mumoshu/kubectl/$(VER)/darwin_amd64/terraform-provider-kubectl_v$(VER)
