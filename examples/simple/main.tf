provider "eksctl" {}

resource "eksctl_cluster" "primary" {
  name = "primary"
  region = "us-east-2"
  spec = <<EOS

nodeGroups:
  - name: ng2
    instanceType: m5.large
    desiredCapacity: 1
EOS
}
