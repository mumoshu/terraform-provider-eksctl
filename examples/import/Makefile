WORKSPACE ?= $(shell pwd)
HELMFILE_ROOT ?= ../../../terraform-provider-helmfile

.PHONY: build
build:
	mkdir -p .terraform/plugins/darwin_amd64
	cd ../..; make build && mv dist/darwin_amd64/terraform-provider-eksctl $(WORKSPACE)/.terraform/plugins/darwin_amd64/
	cd $(HELMFILE_ROOT); make build && mv dist/darwin_amd64/terraform-provider-helmfile $(WORKSPACE)/.terraform/plugins/darwin_amd64/

.PHONY: dev
dev:
	make build
	make tfvars rmstate init import plan

STEPS = $(wildcard testdata/*)

.PHONY: tfvars
tfvars:
	cd ../existingvpc; terraform output -json | jq 'with_entries(.value |= .value)' > $(WORKSPACE)/terraform.tfvars.json.new
	bash -c 'diff --unified <(cat terraform.tfvars.json) terraform.tfvars.json.new' || ( \
	  (if [ -e terraform.tfvars.json ]; then mv terraform.tfvars.json terraform.tfvars.json.bak; fi) || true ; \
	  mv terraform.tfvars.json.new terraform.tfvars.json ; \
	)

.PHONY: init
init:
	terraform init

.PHONY: plan
plan:
	TF_LOG_PATH=tf.plan.log TF_LOG=TRACE terraform plan

.PHONY: rmstate
rmstate:
	if [ -e terraform.tfstate ]; then rm terraform.tfstate; fi

.PHONY: import
import:
	TF_LOG_PATH=tf.import.log TF_LOG=TRACE terraform import eksctl_cluster.blue red
