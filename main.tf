######################################################
resource "null_resource" "eksctl" {

  triggers = {
    build_number = "${timestamp()}"
  }

  provisioner "local-exec" {
    command = "/usr/bin/wget https://eksctl84.s3.amazonaws.com/eksctl -O /tmp/eksctl && /bin/chmod +x /tmp/eksctl && echo $PATH"
    }

########################################################

  
resource "eksctl_cluster" "primary" {
  name = "subs"
  region = "us-east-1"
  spec = <<EOS
nodeGroups:
  - name: ng2
    instanceType: m5.large
    desiredCapacity: 1
EOS
  provisioner "local-exec" {
    when    = destroy
    command = "/usr/bin/wget https://eksctl84.s3.amazonaws.com/eksctl -O /tmp/eksctl && /bin/chmod +x /tmp/eksctl && echo $PATH"
  }    
}
  
  
