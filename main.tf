
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
