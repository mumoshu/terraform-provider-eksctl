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

You use `eksctl_cluster` resource to CRUD your cluster from Terraform.

It's almost like writing and embedding eksctl "cluster.yaml" into `spec` attribute of the Terraform resource definition block, except that some attributes like cluster `name` and `region` has dedicated HCL attributes:

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

The computed field `output` is used to surface the output from `eksctl`. You can use in the string interpolation to produce a useful Terraform output.

## The Goal

My goal for this project is to allow automated canary deployment of a whole K8s cluster via single `terraform apply` run.

That would require a few additional features to this provider, including:

- Ability to attach `eks_cluster` into either ALB or NLB
- Analyze ELB metrics (like 2xx and 5xx count per targetgrous) so that we can postpone `terraform apply` before trying to roll out a broken cluster
- Analyze important pods readiness before rolling out a cluster

[The API is mostly there](https://github.com/mumoshu/terraform-provider-eksctl/blob/master/pkg/resource/cluster/cluster.go#L132-L210), but the implementation of the functionality is still TODO.

## Developing

If you wish to build this yourself, follow the instructions:

	cd terraform-provider-eksctl
	go build

## Acknowledgement

The implementation of this product is highly inspired from [terraform-provider-shell](https://github.com/scottwinkler/terraform-provider-shell). A lot of thanks to the author!
