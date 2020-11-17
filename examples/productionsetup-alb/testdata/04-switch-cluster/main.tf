provider "eksctl" {}
provider "helmfile" {}

variable "region" {
  default = "us-east-2"
  description = "AWS region"
}

variable "vpc_id" {
  description = <<-EOS
  VPC ID. The general recommendation is to use another terraform project to create a vpc and obtain the id by running e.g.
  `terraform output module.vpc.vpc_id`.
  EOS
}

variable "alb_listener_arn" {
  description = <<-EOS
  ARN of the ALB listener to use for canary deployment. For example, this can be obtained from another terraform project
  by running e.g. `terraform output aws_alb_listener.podinfo.arn`.
  EOS
}

variable "vpc_cidr_block" {
  description = <<-EOS
  VPC CIDR. The general recommendation is to use another terraform project to create a VPC and obtain the CIDR by running e.g.
  `terraform output module.vpc.vpc_cidr_block`.
  EOS
}

variable "vpc_private_subnets" {
  type = list(object({
    id  = string
    az = string
    cidr = string
  }))

  default = [
    { id = "example", az="us-west-2a", cidr = "1.2.3.4/24" },
  ]
}

variable "vpc_public_subnets" {
  type = list(object({
    id  = string
    az = string
    cidr = string
  }))

  default = [
    { id = "example", az="us-west-2a", cidr = "1.2.3.4/24" },
  ]
}

variable "vpc_subnet_groups" {
  type = map(
    list(object({
      id  = string
      az = string
      cidr = string
    }))
  )
}

variable "security_group_id" {
  description = <<-EOS
  ID of the security group attached to worker nodes.
  The general recommendation is use another terraform project to create a security group and obtain its id by running e.g.
  `terraform output aws_security_group.public_alb_private_backend.id`.
  EOS
}

provider "aws" {
  version = ">= 2.70.0"
  region = var.region
}

data "aws_availability_zones" "available" {}

locals {
  podinfo_nodeport = 30080
}

resource "aws_lb_target_group" "blue" {
  name = "blue"
  port = local.podinfo_nodeport
  protocol = "HTTP"
  vpc_id = var.vpc_id
}

resource "aws_lb_target_group" "green" {
  name = "green"
  port = local.podinfo_nodeport
  protocol = "HTTP"
  vpc_id = var.vpc_id
}

resource "eksctl_cluster" "blue" {
  eksctl_version = "0.30.0"
  name = "blue"
  region = var.region
  api_version = "eksctl.io/v1alpha5"
  version = "1.18"
  vpc_id = var.vpc_id
  kubeconfig_path = "kubeconfig"
  spec = <<EOS

nodeGroups:
  - name: ng2
    instanceType: m5.large
    desiredCapacity: 1
    targetGroupARNs:
    - ${aws_lb_target_group.blue.arn}
    securityGroups:
      attachIDs:
      - ${var.security_group_id}

iam:
  withOIDC: true
  serviceAccounts:
  - metadata:
      name: reader2
      namespace: default
      labels: {aws-usage: "application"}
    attachPolicyARNs:
    - "arn:aws:iam::aws:policy/AmazonS3ReadOnlyAccess"

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

resource "helmfile_release_set" "blue_myapp_v1" {
  content = file("./helmfile.yaml")
  environment = "default"
  kubeconfig = eksctl_cluster.blue.kubeconfig_path
  depends_on = [
    eksctl_cluster.blue,
  ]
}

resource "eksctl_cluster" "green" {
  eksctl_version = "0.30.0"
  name = "green"
  region = var.region
  api_version = "eksctl.io/v1alpha5"
  version = "1.18"
  vpc_id = var.vpc_id
  kubeconfig_path = "kubeconfig.green"
  spec = <<EOS

nodeGroups:
  - name: ng
    instanceType: m5.large
    desiredCapacity: 1
    targetGroupARNs:
    - ${aws_lb_target_group.green.arn}
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

resource "helmfile_release_set" "green_myapp_v1" {
  content = file("./helmfile.yaml")
  environment = "default"
  kubeconfig = eksctl_cluster.green.kubeconfig_path
  depends_on = [
    eksctl_cluster.green,
  ]
}

resource "eksctl_courier_alb" "myapp" {
  listener_arn = var.alb_listener_arn

  priority = "11"

  step_weight = 10
  step_interval = "10s"

  hosts = ["example.com"]

  destination {
    target_group_arn = aws_lb_target_group.blue.arn

    weight = 0
  }

  destination {
    target_group_arn = aws_lb_target_group.green.arn
    weight = 100
  }

  depends_on = [
    eksctl_cluster.green,
    helmfile_release_set.green_myapp_v1
  ]
}

// The following set of IAM resources allows you use a pod IAM role by using
// service account like:
//
// apiVersion: v1
// kind: ServiceAccount
// metadata:
//   annotations:
//     eks.amazonaws.com/role-arn: arn:aws:iam::ACCOUNT:role/argocd
//     name: argocd-application-controller
//     namespace: default

data "aws_iam_policy_document" "argocd" {
  statement {
    actions = [
      "sts:AssumeRoleWithWebIdentity"]
    effect = "Allow"
    condition {
      test = "StringEquals"
      variable = "${replace(eksctl_cluster.blue.oidc_provider_url, "https://", "")}:sub"
      values = [
        "system:serviceaccount:default:argocd-application-controller"]
    }
    principals {
      identifiers = [
        "${eksctl_cluster.blue.oidc_provider_arn}"]
      type = "Federated"
    }
  }

  statement {
    actions = [
      "sts:AssumeRoleWithWebIdentity"]
    effect = "Allow"
    condition {
      test = "StringEquals"
      variable = "${replace(eksctl_cluster.blue.oidc_provider_url, "https://", "")}:sub"
      values = [
        "system:serviceaccount:default:argocd-server"]
    }
    principals {
      identifiers = [
        "${eksctl_cluster.blue.oidc_provider_arn}"]
      type = "Federated"
    }
  }

  statement {
    actions = [
      "sts:AssumeRoleWithWebIdentity"]
    effect = "Allow"
    condition {
      test = "StringEquals"
      variable = "${replace(eksctl_cluster.green.oidc_provider_url, "https://", "")}:sub"
      values = [
        "system:serviceaccount:default:argocd-application-controller"]
    }
    principals {
      identifiers = [
        "${eksctl_cluster.green.oidc_provider_arn}"]
      type = "Federated"
    }
  }

  statement {
    actions = [
      "sts:AssumeRoleWithWebIdentity"]
    effect = "Allow"
    condition {
      test = "StringEquals"
      variable = "${replace(eksctl_cluster.green.oidc_provider_url, "https://", "")}:sub"
      values = [
        "system:serviceaccount:default:argocd-server"]
    }
    principals {
      identifiers = [
        "${eksctl_cluster.green.oidc_provider_arn}"]
      type = "Federated"
    }
  }
}

resource "aws_iam_role" "argocd" {
  name = "argocd"

  assume_role_policy = data.aws_iam_policy_document.argocd.json
}

resource "aws_iam_role_policy" "argocd" {
  name = "argocd"
  role = aws_iam_role.argocd.id

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:GetCallerIdentity",
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}
EOF
}

output "blue_kubeconfig_path" {
  value = eksctl_cluster.blue.kubeconfig_path
}

output "blue_oidc_provider_url" {
  value = eksctl_cluster.blue.oidc_provider_url
}

output "blue_oidc_provider_arn" {
  value = eksctl_cluster.blue.oidc_provider_arn
}

output "green_kubeconfig_path" {
  value = eksctl_cluster.green.kubeconfig_path
}

output "green_oidc_provider_url" {
  value = eksctl_cluster.green.oidc_provider_url
}

output "green_oidc_provider_arn" {
  value = eksctl_cluster.green.oidc_provider_arn
}
