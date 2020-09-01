# terraform-provider-eksctl

Manage AWS EKS clusters using Terraform and [eksctl](https://github.com/weaveworks/eksctl).

Benefits:

- `terraform apply` to bring up your whole infrastructure.
- No more generating eksctl `cluster.yaml` with Terraform and a glue shell script just for integration between TF and eksctl.

## Installation

**For Terraform 0.12:**

Install the `terraform-provider-eksctl` binary under `.terraform/plugins/${OS}_${ARCH}`, so that the binary is at e.g. `${WORKSPACE}/.terraform/plugins/darwin_amd64/terraform-provider-eksctl`.

You can also install the provider globally under `${HOME}/.terraform.d/plugins/${OS}_${ARCH}`, so that it is available from all the tf workspaces.

**For Terraform 0.13 and later:**

The provider is [available at Terraform Registry](https://registry.terraform.io/providers/mumoshu/eksctl/latest?pollNotifications=true) so you can just add the following to your tf file for installation:

```
terraform {
  required_providers {
    eksctl = {
      source = "mumoshu/eksctl"
      version = "VERSION"
    }
  }
}
```

Please replace `VERSION` with the version number of the provider without the `v` prefix, like `0.3.14`.

## Usage

There is nothing to configure for the provider, so you firstly declare the provider like:

```
provider "eksctl" {}
```

You use `eksctl_cluster` and `eksctl_cluster_deployment` resources to CRUD your clusters from Terraform.

Usually, the former is what you want. It just runs `eksctl` to manage the cluster as exactly as you have declared in your `tf` file.

The latter is, as its name says, for managing a set of `eksctl` clusters in opinionated way.

On `terraform apply`:

- For `eksctl_cluster`, the provider runs a series of `eksctl update [RESOURCE]`. It uses `eksctl delete nodegroup --drain` for deleting nodegroups for high availability.
- For `eksctl_cluster_deployment`, the provider runs `eksctl create` abd a series of `eksctl update [RESOURCE]` and `eksctl delete` depending on the situation. It uses `eksctl delete nodegroup --drain` for deleting nodegroups for high availability.

On `terraform destroy`, the provider runs `eksctl delete`

The computed field `output` is used to surface the output from `eksctl`. You can use in the string interpolation to produce a useful Terraform output.

## Declaring `eksctl_cluster` resource

It's almost like writing and embedding eksctl "cluster.yaml" into `spec` attribute of the Terraform resource definition block, except that some attributes like cluster `name` and `region` has dedicated HCL attributes.

Depending on the scenario, there are a few patterns in how you'de declare a `eksctl_cluster` resource.

- Ephemeral cluster (Don't reuse VPC, subnets, or anything)
- Reuse VPC
- Reuse VPC and subnets
- Reuse VPC, subnets, and ALBs

In general, for any non-ephemeral cluster you must set up the following pre-requisites:

- VPC
- Public/Private subnets
- ALB and listener(s) (Only when you use blue-green cluster deployment) 

### Ephemeral cluster

When you let `eksctl` manage every AWS resource for the cluster, your resource should look like the below:

```hcl-terraform
provider "eksctl" {}

resource "eksctl_cluster" "primary" {
  eksctl_bin = "eksctl-0.20.0"
  name = "primary1"
  region = "us-east-2"
  spec = <<EOS

nodeGroups:
  - name: ng1
    instanceType: m5.large
    desiredCapacity: 1
EOS
}
```

### Reuse VPC

Assuming you've already created a VPC with ID `vpc-09c6c9f579baef3ea`, your resource should look like the below:

```hcl-terraform
provider "eksctl" {}

resource "eksctl_cluster" "vpcreuse1" {
  eksctl_bin = "eksctl-0.20.0"
  name = "vpcreuse1"
  region = "us-east-2"
  vpc_id = "vpc-09c6c9f579baef3ea"
  spec = <<EOS

nodeGroups:
  - name: ng1
    instanceType: m5.large
    desiredCapacity: 1
EOS
}
```

### Reuse VPC and subnets

Assuming you've already created a VPC with ID `vpc-09c6c9f579baef3ea` and a private subnet "subnet-1234",
a public subnet "subnet-2345", your resource should look like the below:

```hcl-terraform
provider "eksctl" {}

resource "eksctl_cluster" "vpcreuse1" {
  eksctl_bin = "eksctl-0.20.0"
  name = "vpcreuse1"
  region = "us-east-2"
  vpc_id = "vpc-09c6c9f579baef3ea"
  spec = <<EOS

vpc:
  cidr: "192.168.0.0/16"       # (optional, must match CIDR used by the given VPC)
  subnets:
    # must provide 'private' and/or 'public' subnets by availibility zone as shown
    private:
      us-east-2a:
        id: "subnet-1234"
        cidr: "192.168.160.0/19" # (optional, must match CIDR used by the given subnet)
    public:
      us-east-2a:
        id: "subnet-2345"
        cidr: "192.168.64.0/19" # (optional, must match CIDR used by the given subnet)

nodeGroups:
  - name: ng1
    instanceType: m5.large
    desiredCapacity: 1
EOS
}
```

### Reuse VPC, subnets, and ALBs

In a production setup, the VPC, subnets, ALB, and listeners should be re-used across revisions of the cluster, so that you can let the provider to switch the cluster revisions in a blue-gree/canary deployment mannaer.

Assuming you've used the [terraform-aws-vpc](https://github.com/terraform-aws-modules/terraform-aws-vpc) module for setting up VPC and subnets, a `eksctl_cluster` resource should usually look like the below:

```hcl-terraform
resource "eksctl_cluster" "primary" {
  eksctl_bin = "eksctl-dev"
  name = "existingvpc2"
  region = "us-east-2"
  api_version = "eksctl.io/v1alpha5"
  version = "1.16"
  vpc_id = module.vpc.vpc_id
  revision = 1
  spec = <<EOS

nodeGroups:
  - name: ng2
    instanceType: m5.large
    desiredCapacity: 1
    securityGroups:
      attachIDs:
      - ${aws_security_group.public_alb_private_backend.id}

iam:
  withOIDC: true
  serviceAccounts: []

vpc:
  cidr: "${module.vpc.vpc_cidr_block}"       # (optional, must match CIDR used by the given VPC)
  subnets:
    # must provide 'private' and/or 'public' subnets by availibility zone as shown
    private:
      ${module.vpc.azs[0]}:
        id: "${module.vpc.private_subnets[0]}"
        cidr: "${module.vpc.private_subnets_cidr_blocks[0]}" # (optional, must match CIDR used by the given subnet)
      ${module.vpc.azs[1]}:
        id: "${module.vpc.private_subnets[1]}"
        cidr: "${module.vpc.private_subnets_cidr_blocks[1]}"  # (optional, must match CIDR used by the given subnet)
      ${module.vpc.azs[2]}:
        id: "${module.vpc.private_subnets[2]}"
        cidr: "${module.vpc.private_subnets_cidr_blocks[2]}"   # (optional, must match CIDR used by the given subnet)
    public:
      ${module.vpc.azs[0]}:
        id: "${module.vpc.public_subnets[0]}"
        cidr: "${module.vpc.public_subnets_cidr_blocks[0]}" # (optional, must match CIDR used by the given subnet)
      ${module.vpc.azs[1]}:
        id: "${module.vpc.public_subnets[1]}"
        cidr: "${module.vpc.public_subnets_cidr_blocks[1]}"  # (optional, must match CIDR used by the given subnet)
      ${module.vpc.azs[2]}:
        id: "${module.vpc.public_subnets[2]}"
        cidr: "${module.vpc.public_subnets_cidr_blocks[2]}"   # (optional, must match CIDR used by the given subnet)
EOS
}
```

## Advanced Features and Use-cases

There's a bunch more settings that helps the app to stay highly available while being recreated, including:

- `kubernetes_resource_deletion_before_destroy`
- `alb_attachment`
- `pods_readiness_check`
- Cluster canary deployment

It's also highly recommended to include `git` configuration and use `eksctl` which includes https://github.com/weaveworks/eksctl/pull/2274 in order to install Flux in an unattended way, so that the cluster has everything deployed on launch. Otherwise blue-green deployments of the cluster doesn't make sense.

Please see the [existingvpc](/examples/existingvpc) example to see how a fully configured eksctl_cluster resource should look like, and the below references for details of each setting.
### Delete Kubernetes resources before destroy

> This option is available only within `eksctl_cluster_deployment` resource

Use `kubernetes_resource_deletion_before_destroy` blocks.

It is useful for e.g.:

- Stopping Flux so that it won't try to install new manifests to fail while the cluster is being terminated
- Stopping pods whose IP addresses are exposed via a headless service and external-dns before the cluster being down, so that stale pod IPs won't remain in the serviced discovery system

```hcl
resource "eksctl_cluster_deployment" "primary" {
  name = "primary"
  region = "us-east-2"

  spec = <<EOS
  - name: ng2
    instanceType: m5.large
    desiredCapacity: 1
EOS

  kubernetes_resource_deletion_before_destroy {
    namespace = "flux"
    kind = "deployment"
    name = "flux"
  }
}
```

## Cluster canary deployment

`courier_alb` resource is used to declaratively and gradually shift traffic among given target groups.

In combination with standard `alb_lb_*` resources and two `eksctl_cluster`, you can conduct a "canary deployment" of the cluster.

> This resource is useful but may be extracted out of this provider in the future.

A `courier_alb` looks like the below:

```
resource "eksctl_courier_alb" "my_alb_courier" {
  listener_arn = "<alb listener arn>"

  priority = "10"
  
  destination {
    target_group_arn = "<target group arn current>"

    weight = 0
  }

  destination {
    target_group_arn = "<target group arn next>"
    weight = 100
  }

  cloudwatch_metric {
    name = "http_errors_cw"

    # it will query from <now - 60 sec> to now, every 60 sec
    interval = "1m"

    max = 50

    query = "<QUERY>"
  }

  datadog_metric {
    name = "http_errors_dd"

    # it will query from <now - 60 sec> to now, every 60 sec
    interval = "1m"

    max = 50

    query = "<QUERY>"
  }
}
```

Let's say you want to serve your web service on port 80 of your internet-facing ALB. You'll start with a `alb`, `alb_listener`, and two `alb_target_group`s and two `eksctl-cluster`.

The below is the initial deployment with two clusters `blue` and `green`, where the traffic is 100% forwarded to `blue` and `helmfile` is used to deploy Helm charts to `blue`:

```hcl-terraform
resource "aws_alb" "alb" {
  name = "alb"
  security_groups = [
    aws_security_group.public_alb.id
  ]
  subnets = module.vpc.public_subnets
  internal = false
  enable_deletion_protection = false
}

resource "aws_alb_listener" "mysvc" {
  port = 80
  protocol = "HTTP"
  load_balancer_arn = aws_alb.alb.arn
  default_action {
    type = "fixed-response"
    fixed_response {
      content_type = "text/plain"
      status_code = "404"
      message_body = "Nothing here"
    }
  }
}

resource "aws_lb_target_group" "blue" {
  name = "tg1"
  port = 30080
  protocol = "HTTP"
  vpc_id = module.vpc.vpc_id
}

resource "aws_lb_target_group" "green" {
  name = "tg2"
  port = 30080
  protocol = "HTTP"
  vpc_id = module.vpc.vpc_id
}

resource "eksctl_cluster" "blue" {
  name = "blue"
  region = "us-east-2"
  api_version = "eksctl.io/v1alpha5"
  version = "1.15"
  vpc_id = module.vpc.vpc_id
  spec = <<EOS
nodeGroups:
  - name: ng2
    instanceType: m5.large
    desiredCapacity: 1
    targetGroupARNs:
    - ${aws_lb_target_group.blue.arn}
EOS
}

resource "eksctl_cluster" "green" {
  name = "green"
  region = "us-east-2"
  api_version = "eksctl.io/v1alpha5"
  version = "1.16"
  vpc_id = module.vpc.vpc_id
  spec = <<EOS
nodeGroups:
  - name: ng2
    instanceType: m5.large
    desiredCapacity: 1
    targetGroupARNs:
    - ${aws_lb_target_group.green.arn}
EOS
}

resource "helmfile_release_set" "myapps" {
  content = file("./helmfile.yaml")
  environment = "default"
  environment_variables = {
    KUBECONFIG = eksctl_cluster.blue.kubeconfig_path
  }
  depends_on = [
    eksctl_cluster.blue
  ]
}

resource "eksctl_courier_alb" "my_alb_courier" {
  listener_arn = aws_alb_listener.mysvc.arn

  priority = "11"

  destination {
    target_group_arn = aws_lb_target_group.blue.arn

    weight = 100
  }

  destination {
    target_group_arn = aws_lb_target_group.green.arn
    weight = 0
  }

  depends_on = [
    helmfile_release_set.myapps
  ]
}
```

Wanna make a critical change to `blue`, without fearing downtime?

Rethink and update `green` instead, while changing `courier_alb`'s `weight` so that the traffic is forwarded to `green` only after
the cluster is successfully updated:

```hcl-terraform

resource "helmfile_release_set" "myapps" {
  content = file("./helmfile.yaml")
  environment = "default"
  environment_variables = {
    # It was `eksctl_cluster.blue.kubeconfig_path` before
    KUBECONFIG = eksctl_cluster.green.kubeconfig_path
  }
  depends_on = [
    # This was eksctl_cluster.blue before the update
    eksctl_cluster.green
  ]
}

resource "eksctl_courier_alb" "my_alb_courier" {
  listener_arn = aws_alb_listener.mysvc.arn

  priority = "11"

  destination {
    target_group_arn = aws_lb_target_group.blue.arn
    # This was 100 before the update
    weight = 0
  }

  destination {
    target_group_arn = aws_lb_target_group.green.arn
    # This was 0 before the update
    weight = 100
  }

  depends_on = [
    helmfile_release_set.myapps
  ]
}
```

This instructs Terraform to:

- Update `eksctl_cluster.green`
- Run `helmfile` against the `green` cluster to have all the Helm charts deployed
- Gradually shift the traffic from the previous `blue` cluster to the updated `green` cluster.

In addition, you can add `cloudwatch_metric`s and/or `datadog_metric`s to `courier_alb`'s `destinations`, so that the provider runs canary analysis to determine
whether it should continue shifting the traffic.

## The Goal

My goal for this project is to allow automated canary deployment of a whole K8s cluster via single `terraform apply` run.

That would require a few additional features to this provider, including:

- [x] Ability to attach `eks_cluster` to ALB
- [ ] Analyze ALB metrics (like 2xx and 5xx count per targetgrous) so that we can postpone `terraform apply` before trying to roll out a broken cluster
- [x] Analyze important pods readiness before rolling out a cluster
  - Implemented. Use `pods_readiness_check` blocks.
- [ ] Analyze Datadog metrics (like request success/error rate, background job sucess/error rate, etc.) before rolling out a new cluster.
- [x] Specify default K8s resource manifests to be applied on the cluster
  - [The new kubernetes provider](https://www.hashicorp.com/blog/deploy-any-resource-with-the-new-kubernetes-provider-for-hashicorp-terraform/) doesn't help it. What we need is ability to apply manifests after the cluster creation but before completing update on the `eks_cluster` resource. With the kubernetes provider, the manifests are applied AFTER the `eksctl_cluster` update is done, which isn't what we want.
  - Implemented. Use the `manifests` attribute.
- [ ] Ability to attach `eks_cluster` to NLB

`terraform-provider-eksctl` is my alternative to the imaginary `eksctl-controller`.

I have been long considered about developing a K8s controller that allows you to manage eksctl cluster updates fully declaratively via a K8s CRD. The biggest pain point of that model is you still need a multi-cluster control-plane i.e. a "management" K8s cluster, which adds additional operational/maintenance cost for us.

If I implement the required functionality to a terraform provider, we don't need an additional K8s cluster for management, as the state is already stored in the terraform state and the automation is aleady done with `Atlantis`, Terraform Enterprise, or any CI systems like CircleCI, GitHub Actions, etc.

As of today, [the API is mostly there](https://github.com/mumoshu/terraform-provider-eksctl/blob/master/pkg/resource/cluster/cluster.go#L132-L210), but the implementation of the functionality is still TODO.

## Developing

If you wish to build this yourself, follow the instructions:

```
$ cd terraform-provider-eksctl
$ go build
```

There's also a convenient Make target for installing the provider into the global tf providers directory:

```
$ make install
```

The above will install the provider's binary under `${HOME}/.terraform.d/plugins/${OS}_${ARCH}`.

## Acknowledgement

The implementation of this product is highly inspired from [terraform-provider-shell](https://github.com/scottwinkler/terraform-provider-shell). A lot of thanks to the author!
