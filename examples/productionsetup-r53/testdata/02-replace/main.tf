provider "eksctl" {}
provider "helmfile" {}

provider "aws" {
  version = ">= 2.70.0"
  region = var.region
}

data "aws_availability_zones" "available" {}

locals {
  podinfo_nodeport = 30080
}

resource "aws_lb" "r53blue" {
  name = "r53blue"
  internal = false
  load_balancer_type = "network"
  subnets = var.vpc_public_subnets_ids

  enable_deletion_protection = false

  tags = {
    Environment = "production"
  }
}

resource "aws_lb" "r53green" {
  name = "r53green"
  internal = false
  load_balancer_type = "network"
  subnets = var.vpc_public_subnets_ids

  enable_deletion_protection = false

  tags = {
    Environment = "production"
  }
}

resource "aws_lb_target_group" "r53blue" {
  name = "r53blue"
  port = local.podinfo_nodeport
  protocol = "HTTP"
  vpc_id = var.vpc_id
}

resource "aws_lb_target_group" "r53green" {
  name = "r53green"
  port = local.podinfo_nodeport
  protocol = "HTTP"
  vpc_id = var.vpc_id
}

resource "aws_lb_listener" "r53blue" {
  load_balancer_arn = aws_lb.r53blue.arn
  port = 30080
  protocol = "TCP"
  default_action {
    target_group_arn = aws_lb_target_group.r53blue.arn
    type = "forward"
  }
}

resource "aws_lb_listener" "r53green" {
  load_balancer_arn = aws_lb.r53green.arn
  port = 30080
  protocol = "TCP"
  default_action {
    target_group_arn = aws_lb_target_group.r53blue.arn
    type = "forward"
  }
}

resource "aws_route53_record" "r53blue" {
  zone_id = var.route53_zone_id
  name    = var.route53_record_name
  type    = "A"

  set_identifier = "blue"

  alias {
    name                   = aws_lb.r53blue.dns_name
    zone_id                = aws_lb.r53blue.zone_id
    evaluate_target_health = false
  }

  weighted_routing_policy {
    weight = 1
  }

  lifecycle {
    ignore_changes = [
      weighted_routing_policy,
    ]
  }
}

resource "aws_route53_record" "r53green" {
  zone_id = var.route53_zone_id
  name    = var.route53_record_name
  type    = "A"

  set_identifier = "green"

  alias {
    name                   = aws_lb.r53blue.dns_name
    zone_id                = aws_lb.r53blue.zone_id
    evaluate_target_health = false
  }

  weighted_routing_policy {
    weight = 0
  }

  lifecycle {
    ignore_changes = [
      weighted_routing_policy,
    ]
  }
}

resource "eksctl_cluster" "r53blue" {
  eksctl_bin = "eksctl-dev"
  name = "r53blue"
  region = var.region
  api_version = "eksctl.io/v1alpha5"
  version = "1.16"
  vpc_id = var.vpc_id
  spec = <<EOS

nodeGroups:
  - name: ng
    instanceType: m5.large
    desiredCapacity: 1
    targetGroupARNs:
    - ${aws_lb_target_group.r53blue.arn}
    securityGroups:
      attachIDs:
      - ${var.security_group_id}

iam:
  withOIDC: true
  serviceAccounts: []

vpc:
  cidr: "${var.vpc_cidr_block}"       # (optional, must match CIDR used by the given VPC)
  subnets:
    %{~ for group in keys(var.vpc_subnet_groups) }
    ${group}:
      %{~ for subnet in var.vpc_subnet_groups[group] }
      ${subnet.az}:
        id: "${subnet.id}"
        cidr: "${subnet.cidr}"
      %{ endfor ~}
    %{ endfor ~}

git:
  repo:
    url: "git@github.com:mumoshu/gitops-demo.git"
    branch: master
    fluxPath: "flux/"
    user: "gitops"
    email: "gitops@myorg.com"
    ## Uncomment this when `commitOperatorManifests: true`
    #privateSSHKeyPath: /path/to/your/ssh/key
  operator:
    commitOperatorManifests: false
    namespace: "flux"
    readOnly: true
EOS
}

resource "eksctl_cluster" "r53green" {
  eksctl_bin = "eksctl-dev"
  name = "r53green"
  region = var.region
  api_version = "eksctl.io/v1alpha5"
  version = "1.16"
  vpc_id = var.vpc_id
  spec = <<EOS

nodeGroups:
  - name: ng
    instanceType: m5.large
    desiredCapacity: 1
    targetGroupARNs:
    - ${aws_lb_target_group.r53green.arn}
    securityGroups:
      attachIDs:
      - ${var.security_group_id}

iam:
  withOIDC: true
  serviceAccounts: []

vpc:
  cidr: "${var.vpc_cidr_block}"       # (optional, must match CIDR used by the given VPC)
  subnets:
    %{~ for group in keys(var.vpc_subnet_groups) }
    ${group}:
      %{~ for subnet in var.vpc_subnet_groups[group] }
      ${subnet.az}:
        id: "${subnet.id}"
        cidr: "${subnet.cidr}"
      %{ endfor ~}
    %{ endfor ~}

git:
  repo:
    url: "git@github.com:mumoshu/gitops-demo.git"
    branch: master
    fluxPath: "flux/"
    user: "gitops"
    email: "gitops@myorg.com"
    ## Uncomment this when `commitOperatorManifests: true`
    #privateSSHKeyPath: /path/to/your/ssh/key
  operator:
    commitOperatorManifests: false
    namespace: "flux"
    readOnly: true
EOS
}

resource "helmfile_release_set" "r53blue_myapp_v1" {
  content = file("./helmfile.yaml")
  environment = "default"
  environment_variables = {
    KUBECONFIG = eksctl_cluster.r53blue.kubeconfig_path
  }
  depends_on = [
    eksctl_cluster.r53blue,
  ]
}

resource "helmfile_release_set" "r53green_myapp_v1" {
  content = file("./helmfile.yaml")
  environment = "default"
  environment_variables = {
    KUBECONFIG = eksctl_cluster.r53green.kubeconfig_path
  }
  depends_on = [
    eksctl_cluster.r53green,
  ]
}

resource "eksctl_courier_route53_record" "myapp" {
  zone_id = var.route53_zone_id
  name = var.route53_record_name

  step_weight = 10
  step_interval = "10s"

  destination {
    set_identifier = aws_route53_record.r53blue.set_identifier

    weight = 0
  }

  destination {
    set_identifier = aws_route53_record.r53green.set_identifier

    weight = 100
  }

  depends_on = [
    eksctl_cluster.r53green,
    helmfile_release_set.r53green_myapp_v1
  ]
}

output "blue_kubeconfig_path" {
  value = eksctl_cluster.r53blue.kubeconfig_path
}

output "green_kubeconfig_path" {
  value = eksctl_cluster.r53green.kubeconfig_path
}
