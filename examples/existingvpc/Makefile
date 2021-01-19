#WORKSPACE ?= $(shell pwd)
#HELMFILE_ROOT ?= ../../../terraform-provider-helmfile

include ../lib/lib.mk

.PHONY: write-kubeconfig
write-kubeconfig:
	KUBECONFIG=kubeconfig eksctl-dev utils write-kubeconfig --cluster gitops1

.PHONY: apply
apply: manifests/metrics-server-v0.3.6
	terraform init && terraform plan && TF_LOG=TRACE terraform apply -auto-approve

.PHONY: dev
dev:
	cd ../..; make build && mv dist/darwin_amd64/terraform-provider-eksctl $(WORKSPACE)/.terraform/plugins/darwin_amd64/
	cd $(HELMFILE_ROOT); make build && mv dist/darwin_amd64/terraform-provider-helmfile $(WORKSPACE)/.terraform/plugins/darwin_amd64/
	TF_LOG_PATH=tf.log TF_LOG=TRACE make apply

.PHONY: plan
plan:
	terraform init && TF_LOG=DEBUG terraform plan

.PHONY: destroy
destroy:
	terraform init && TF_LOG=DEBUG terraform destroy -auto-approve

.PHONY: recreate
recreate: destroy apply

.PHONY: e2e
e2e:
	make apply destroy

METRICS_SERVER_VER ?= v0.3.6

manifests/metrics-server-$(METRICS_SERVER_VER):
	echo Downloading $(METRICS_SERVER_VER)
	mkdir -p manifests/metrics-server-$(METRICS_SERVER_VER)
	curl -L https://github.com/kubernetes-sigs/metrics-server/releases/download/$(METRICS_SERVER_VER)/components.yaml > manifests/metrics-server-$(METRICS_SERVER_VER)/all.yaml
