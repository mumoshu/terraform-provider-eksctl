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
