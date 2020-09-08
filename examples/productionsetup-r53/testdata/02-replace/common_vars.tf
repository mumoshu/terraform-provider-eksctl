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

variable "vpc_public_subnets_ids" {
  type = list(string)
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
