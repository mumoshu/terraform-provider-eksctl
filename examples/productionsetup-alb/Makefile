WORKSPACE ?= $(shell pwd)
HELMFILE_ROOT ?= ../../../terraform-provider-helmfile

.PHONY: dev
dev:
	mkdir -p .terraform/plugins/darwin_amd64
	cd ../..; make build && mv dist/darwin_amd64/terraform-provider-eksctl $(WORKSPACE)/.terraform/plugins/darwin_amd64/
	cd $(HELMFILE_ROOT); make build && mv dist/darwin_amd64/terraform-provider-helmfile $(WORKSPACE)/.terraform/plugins/darwin_amd64/
	make testdata/01-bootstrap init plan apply testdata/02-replace plan apply

STEPS = $(wildcard testdata/*)

.PHONY: $(STEPS)
.SECONDEXPANSION:
$(STEPS):%:$$(wildcard %/*.tf)
	@echo Preparing $@
	cp $< .

.PHONY: init
init:
	terraform init

.PHONY: plan
plan:
	TF_LOG_PATH=tf.plan.log TF_LOG=TRACE terraform plan

.PHONY: apply
apply:
	TF_LOG_PATH=tf.apply.log TF_LOG=TRACE terraform apply -auto-approve

.PHONY: destroy
destroy:
	TF_LOG_PATH=tf.destroy.log TF_LOG=TRACE terraform destroy -auto-approve