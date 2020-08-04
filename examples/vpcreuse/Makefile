.PHONY: write-kubeconfig
write-kubeconfig:
	KUBECONFIG=kubeconfig eksctl-0.20.0 utils write-kubeconfig --cluster vpcreuse1

.PHONY: smoke
smoke:
	cd ../.. && make clean install
	terraform init
	terraform plan
	terraform apply -auto-approve
