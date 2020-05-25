# terraform-provider-eksctl

Manage AWS EKS clusters using Terraform and [eksctl](https://github.com/weaveworks/eksctl).

Benefits:

- `terraform apply` to bring up your whole infrastructure.
- No more generating eksctl `cluster.yaml` with Terraform and a glue shell script just for integration between TF and eksctl.

## Installation

Install the `terraform-provider-eksctl` binary under `terraform.d/plugins/${OS}_${ARCH}`.

There's a convenient Make target for that so you can just run:

```
$ make install
```

## Usage

There is nothing to configure for the provider, so you firstly declare the provider like:

```
provider "eksctl" {}
```

You use `eksctl_clsuter` resource to CRUD your cluster from Terraform.

It's almost like writing and embedding eksctl "cluster.yaml" into `spec` attribuet of the Terraform resource definition block, except that some attributes like cluster `name` and `region` has dedicated HCL attributes: 

```
resource "eksctl_cluster" "primary" {
  name = "primary"
  region = "us-east-2"

  spec = <<EOS
  - name: ng2
    instanceType: m5.large
    desiredCapacity: 1
EOS
```

On `terraform apply`, the provider runs `eksctl create`, `eksctl update` and `eksctl delete` depending on the situation. It uses `eksctl delete nodegroup --drain` for deleting nodegroups for high availability.

On `terraform destroy`, the provider runs `eksctl delete`

The computed field `output` is used to surface the output from Helmfile. You can use in the string interpolation to produce a useful Terraform output.

## Developing

If you wish to build this yourself, follow the instructions:

	cd terraform-provider-eksctl
	go build

## Acknowledgement

The implementation of this product is highly inspired from [terraform-provider-shell](https://github.com/scottwinkler/terraform-provider-shell). A lot of thanks to the author!
