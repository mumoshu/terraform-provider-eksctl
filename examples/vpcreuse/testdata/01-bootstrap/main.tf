provider "eksctl" {}

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

resource "eksctl_cluster" "vpcreuse1" {
  eksctl_bin = "eksctl-0.20.0"
  name = "vpcreuse1"
  region = "us-east-2"
  vpc_id = var.vpc_id
  spec = <<EOS

vpc:
  clusterEndpoints:
    privateAccess: true
    publicAccess: true
  publicAccessCIDRs: ["1.1.1.1/32", "2.2.2.2/32"]
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

EOS
}
