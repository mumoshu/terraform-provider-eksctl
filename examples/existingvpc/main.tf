provider "eksctl" {}
provider "helmfile" {}
provider "kubectl" {}

terraform {
  required_providers {
    eksctl = {
      source = "mumoshu/eksctl"
      version = "0.0.1"
    }

    helmfile = {
      source = "mumoshu/helmfile"
      version = "0.0.1"
    }

    kubectl = {
      source = "mumoshu/kubectl"
      version = "0.0.1"
    }
  }
}


variable "region" {
  default = "us-east-2"
  description = "AWS region"
}

variable "role_arn" {
}

variable "myip" {

}

provider "aws" {
  version = ">= 2.70.0"
  region = "us-east-2"
  ignore_tags {
    key_prefixes = [
      "kubernetes.io/cluster/"]
  }
}

data "aws_availability_zones" "available" {}

module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
  version = "2.6.0"

  name = "training-vpc"
  cidr = "192.168.0.0/16"
  azs = data.aws_availability_zones.available.names
  private_subnets = [
    "192.168.96.0/19",
    "192.168.128.0/19",
    "192.168.160.0/19"]
  public_subnets = [
    "192.168.0.0/19",
    "192.168.32.0/19",
    "192.168.64.0/19"]
  enable_nat_gateway = true
  single_nat_gateway = true
  enable_dns_hostnames = true

  public_subnet_tags = {
    // https://docs.aws.amazon.com/eks/latest/userguide/network_reqs.html#vpc-subnet-tagging
    "kubernetes.io/role/elb" = "1"
  }

  private_subnet_tags = {
    // https://docs.aws.amazon.com/eks/latest/userguide/network_reqs.html#vpc-subnet-tagging
    "kubernetes.io/role/internal-elb" = "1"
  }

  // The following tags are created/deleted by the provider:
  //
  //  tags = {
  //    "kubernetes.io/cluster/${local.cluster_name}" = "shared"
  //  }
  //
  //  public_subnet_tags = {
  //    "kubernetes.io/cluster/${local.cluster_name}" = "shared"
  //    "kubernetes.io/role/elb"                      = "1"
  //  }
  //
  //  private_subnet_tags = {
  //    "kubernetes.io/cluster/${local.cluster_name}" = "shared"
  //    "kubernetes.io/role/internal-elb"             = "1"
  //  }

  // Wd wanna prevent provider-added tags to result in detected changes on plan.
  // But terraform doesn't allow injecting `lifecycle` block into module-managed resources.
  //
  //lifecycle {
  //  ignore_changes = ["tags.\"kubernetes.io/cluster/\".%"]
  //}
  //
  // Instead, we use ignore_tags provided by the aws provider. See:
  // - https://github.com/terraform-aws-modules/terraform-aws-vpc/issues/188#issuecomment-558627861
  // - https://www.terraform.io/docs/providers/aws/#ignore_tags-configuration-block
}

locals {
  myip_cidr = "${var.myip}/32"
  podinfo_nodeport = 30080
}

resource "aws_security_group" "allow_http_from_me" {
  name = "allow_http_from_me"
  description = "Allow inbound traffic"
  vpc_id = "${module.vpc.vpc_id}"

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
  name = "public_alb"
  description = "Allow healthcheck against private and public nodes"
  vpc_id = "${module.vpc.vpc_id}"

  egress {
    from_port = 0
    to_port = 0
    protocol = "-1"
    cidr_blocks = [
      module.vpc.vpc_cidr_block]
  }

  tags = {
    Name = "public_alb"
  }
}

resource "aws_security_group" "public_alb_private_backend" {
  name = "public_alb_private_backend"
  description = "Allow healthcheck against nodes"
  vpc_id = "${module.vpc.vpc_id}"

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
  name = "existingvpc2"
  security_groups = [
    aws_security_group.public_alb.id,
    aws_security_group.allow_http_from_me.id]
  subnets = module.vpc.public_subnets
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

resource "aws_lb_target_group" "tg1" {
  name = "tg1"
  port = 30080
  protocol = "HTTP"
  vpc_id = module.vpc.vpc_id
}

resource "aws_lb_target_group" "tg2" {
  name = "tg2"
  port = 30080
  protocol = "HTTP"
  vpc_id = module.vpc.vpc_id
}

resource "eksctl_cluster" "red" {
  assume_role {
    role_arn = var.role_arn
  }
  eksctl_bin = "eksctl"
  name = "red2"
  region = var.region
  api_version = "eksctl.io/v1alpha5"
  version = "1.16"
  vpc_id = module.vpc.vpc_id
  kubeconfig_path = "mykubeconfig"
  spec = <<EOS

nodeGroups:
  - name: ng2
    instanceType: m5.large
    desiredCapacity: 1
    targetGroupARNs:
    - ${aws_lb_target_group.tg2.arn}
    securityGroups:
      attachIDs:
      - ${aws_security_group.public_alb_private_backend.id}

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
  clusterEndpoints:
    privateAccess: true
    publicAccess: true
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

  depends_on = [
    module.vpc]
}

resource "eksctl_nodegroup" "ng1" {
  assume_role {
    role_arn = var.role_arn
  }
  name = "ng1"
  region = eksctl_cluster.red.region
  cluster = eksctl_cluster.red.name
  nodes_min = 1
  nodes = 1
}
//
//resource "eksctl_courier_alb" "my_alb_courier" {
//  listener_arn = aws_alb_listener.podinfo.arn
//
//  priority = "11"
//
//  step_weight = 10
//  step_interval = "5s"
//
//  hosts = [
//    "exmaple.com"]
//
//  destination {
//    target_group_arn = aws_lb_target_group.tg1.arn
//
//    weight = 100
//  }
//
//  destination {
//    target_group_arn = aws_lb_target_group.tg2.arn
//    weight = 0
//  }
//
//  depends_on = [
//    eksctl_cluster.red,
////    helmfile_release_set.mystack1
//  ]
//}

resource "helmfile_release_set" "mystack1" {
  aws_region = var.region
  aws_assume_role {
    role_arn = var.role_arn
  }
  content = file("./helmfile.yaml")
  environment = "default"
  kubeconfig = eksctl_cluster.red.kubeconfig_path
  depends_on = [
    eksctl_cluster.red,
  ]
}

resource "kubectl_ensure" "meta" {
  aws_region = var.region
  aws_assume_role {
    role_arn = var.role_arn
  }

  kubeconfig = eksctl_cluster.red.kubeconfig_path

  namespace = "kube-system"
  resource = "configmap"
  name = "aws-auth"

  labels = {
    "key1" = "one"
    "key2" = "two"
  }

  annotations = {
    "key3" = "three"
    "key4" = "four"
  }
}

output "kubeconfig_path" {
  value = eksctl_cluster.red.kubeconfig_path
}

output "alb_listener_arn" {
  value = aws_alb_listener.podinfo.arn
}

output "vpc_id" {
  value = module.vpc.vpc_id
}

output "vpc_public_subnets_ids" {
  value = module.vpc.public_subnets
}

output "vpc_private_subnet_ids" {
  value = module.vpc.private_subnets
}

output "vpc_public_subnet_cidr_blocks" {
  value = module.vpc.public_subnets_cidr_blocks
}

output "vpc_private_subnet_cidr_blocks" {
  value = module.vpc.private_subnets_cidr_blocks
}

output "vpc_subnet_azs" {
  value = module.vpc.azs
}

output "vpc_cidr_block" {
  value = module.vpc.vpc_cidr_block
}

output "vpc_subnet_groups" {
  value = {
    "public" = [
    for i in range(length(module.vpc.azs)):
    {
      cidr = module.vpc.public_subnets_cidr_blocks[i],
      az = module.vpc.azs[i],
      id = module.vpc.public_subnets[i],
    }
    ],
    "private" = [
    for i in range(length(module.vpc.azs)):
    {
      cidr = module.vpc.private_subnets_cidr_blocks[i],
      az = module.vpc.azs[i],
      id = module.vpc.private_subnets[i],
    }
    ]
  }
}

output "security_group_id" {
  value = aws_security_group.public_alb_private_backend.id
}
