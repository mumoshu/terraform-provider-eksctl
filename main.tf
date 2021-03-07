
terraform {
  required_providers {
    eksctl = {
      source = "mumoshu/eksctl"
      version = "0.15.1"
    }
  }
}

provider "aws" {
}

resource "eksctl_cluster" "primary" {
  name = "subs"
  region = "us-east-1"
  spec = <<EOS
nodeGroups:
  - name: ng2
    instanceType: m5.large
    desiredCapacity: 1
EOS
}
