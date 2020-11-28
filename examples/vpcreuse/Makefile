WORKSPACE ?= $(shell pwd)
HELMFILE_ROOT ?= ../../../terraform-provider-helmfile
TERRAFORM ?= terraform

.PHONY: build
build: VER=0.0.1
build:
	mkdir -p .terraform/plugins/darwin_amd64
	cd ../..; make build
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

.PHONY: dev
dev:
	make build
	make testdata/01-bootstrap init tfvars plan apply
	make testdata/02-nodegroup-create-then-delete plan apply
	make destroy

STEPS = $(wildcard testdata/*)

.PHONY: tfvars
tfvars:
	cd ../existingvpc; terraform output -json | jq 'with_entries(.value |= .value)' > $(WORKSPACE)/terraform.tfvars.json.new
	bash -c 'diff --unified <(cat terraform.tfvars.json) terraform.tfvars.json.new' || ( \
	  (if [ -e terraform.tfvars.json ]; then mv terraform.tfvars.json terraform.tfvars.json.bak; fi) || true ; \
	  mv terraform.tfvars.json.new terraform.tfvars.json ; \
	)

.PHONY: $(STEPS)
.SECONDEXPANSION:
$(STEPS):%:$$(wildcard %/*.tf)
	@echo Preparing $@
	cp $< .
	echo $(shell basename $@) > CURRENT

.PHONY: init
init:
	$(TERRAFORM) init

.PHONY: testout
testout:
	mkdir -p testout/$$(cat CURRENT)

.PHONY: plan
plan: testout
	if [ -e testout/$$(cat CURRENT)/tf.plan.log ]; then rm testout/$$(cat CURRENT)/tf.plan.log; fi
	TF_LOG_PATH=testout/$$(cat CURRENT)/tf.plan.log TF_LOG=TRACE $(TERRAFORM) plan -var aws_account_id=$(AWS_ACCOUNT_ID) -var myip=$(MYIP)

.PHONY: apply
apply: testout
	if [ -e testout/$$(cat CURRENT)/tf.apply.log ]; then rm testout/$$(cat CURRENT)/tf.apply.log; fi
	TF_LOG_PATH=testout/$$(cat CURRENT)/tf.apply.log TF_LOG=TRACE $(TERRAFORM) apply -auto-approve -var aws_account_id=$(AWS_ACCOUNT_ID) -var myip=$(MYIP)

.PHONY: destroy
destroy: testout
	if [ -e testout/$$(cat CURRENT)/tf.destroy.log ]; then rm testout/$$(cat CURRENT)/tf.destroy.log; fi
	TF_LOG_PATH=testout/$$(cat CURRENT)/tf.destroy.log TF_LOG=TRACE $(TERRAFORM) destroy -auto-approve -var aws_account_id=$(AWS_ACCOUNT_ID) -var myip=$(MYIP)
