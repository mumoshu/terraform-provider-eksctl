//terraform {
//  required_providers {
//    eksctl = {
//      source = "mumoshu/eksctl"
//      version = "0.10.0"
//    }
//  }
//}

provider "eksctl" {}

provider "aws" {
  region = "us-east-2"
}

variable "vpc_id" {
  description = <<-EOS
  VPC ID. The general recommendation is to use another terraform project to create a vpc and obtain the id by running e.g.
  `terraform output module.vpc.vpc_id`.
  EOS
}

variable "vpc_cidr_block" {
  description = <<-EOS
  VPC CIDR. The general recommendation is to use another terraform project to create a VPC and obtain the CIDR by running e.g.
  `terraform output module.vpc.vpc_cidr_block`.
  EOS
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

variable "aws_account_id" {
  type = string
}

variable "security_group_id" {
  type = string
}

variable "vpc_public_subnets_ids" {
  type = list(string)
}

resource "eksctl_cluster" "vpcreuse1" {
  eksctl_version = "0.30.0"
  version = "1.18"
  name = "vpcreuse2"
  region = "us-east-2"
  tags = {
    foo = "bar"
  }
  vpc_id = var.vpc_id
  spec = <<EOS

vpc:
  clusterEndpoints:
    privateAccess: true
    publicAccess: true
#  publicAccessCIDRs: ["203.141.59.1/32", "126.182.249.162/32", "121.2.182.55/32"]
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

nodeGroups:
  - name: ng1
    instanceType: m5.large
    desiredCapacity: 1
    iam:
      withAddonPolicies:
        appMesh: true
        appMeshPreview: true
    targetGroupARNs:
    - ${aws_lb_target_group.vpcreuse1.arn}
    securityGroups:
      attachIDs:
      - ${aws_security_group.public_alb_private_backend.id}

iam:
  withOIDC: true
EOS

  iam_identity_mapping {
    groups = ["system:masters"]
    iamarn = "arn:aws:iam::${var.aws_account_id}:role/argocd"
    username = "argocd-manager"
  }
}

resource "aws_security_group_rule" "allow_k8s_api_access_from_argocd" {
  type              = "ingress"
  to_port           = 443
  protocol          = "-1"
  from_port         = 0
  security_group_id = eksctl_cluster.vpcreuse1.security_group_ids[0]
  source_security_group_id = var.security_group_id
  description       = "Allow private k8s api endpoint access from argocd to my control-plane. Requires that ${var.security_group_id} is associated to all the worker nodes running ArgoCD application controller"
}

variable "myip" {
  type = string
}

locals {
  myip_cidr = "${var.myip}/32"
}

resource "aws_security_group" "allow_http_from_me" {
  name = "vpcreuse"
  description = "Allow inbound traffic"
  vpc_id = "${var.vpc_id}"

  ingress {
    description = "HTTTP from me"
    from_port = 80
    to_port = 80
    protocol = "tcp"
    cidr_blocks = [
      local.myip_cidr]
  }

  tags = {
    Name = "allow_http_from_me"
  }
}

resource "aws_security_group" "public_alb" {
  name = "vpcreuse_public_alb"
  description = "Allow healthcheck against private and public nodes"
  vpc_id = "${var.vpc_id}"

  egress {
    from_port = 0
    to_port = 0
    protocol = "-1"
    cidr_blocks = [
      var.vpc_cidr_block]
  }

  tags = {
    Name = "public_alb"
  }
}

resource "aws_security_group" "public_alb_private_backend" {
  name = "vpcreuse_public_alb_private_backend"
  description = "Allow healthcheck against nodes"
  vpc_id = "${var.vpc_id}"

  ingress {
    description = "Allow healthchecking and forwarding from alb"
    from_port = 0
    to_port = 0
    protocol = "-1"
    security_groups = [
      aws_security_group.public_alb.id]
  }

  tags = {
    Name = "public_alb_private_backend"
  }
}

resource "aws_alb" "alb" {
  name = "vpcreuse"
  security_groups = [
    aws_security_group.public_alb.id,
    aws_security_group.allow_http_from_me.id]
  subnets = var.vpc_public_subnets_ids
  internal = false
  enable_deletion_protection = false
}

resource "aws_alb_listener" "podinfo" {
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

resource "aws_alb_listener_rule" "static" {
  listener_arn = aws_alb_listener.podinfo.arn
  priority     = 100

  action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.vpcreuse1.arn
  }

  condition {
    path_pattern {
      values = ["/*"]
    }
  }
}

resource "aws_lb_target_group" "vpcreuse1" {
  name = "vpcreuse1"
  port = 30080
  protocol = "HTTP"
  vpc_id = var.vpc_id
}
