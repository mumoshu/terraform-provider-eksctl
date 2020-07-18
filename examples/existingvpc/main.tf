provider "eksctl" {}

variable "region" {
  default     = "us-east-2"
  description = "AWS region"
}

provider "aws" {
  version = ">= 2.70.0"
  region  = "us-east-2"
  ignore_tags {
    key_prefixes = ["kubernetes.io/cluster/"]
  }
}

data "aws_availability_zones" "available" {}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "2.6.0"

  name                 = "training-vpc"
  cidr                 = "192.168.0.0/16"
  azs                  = data.aws_availability_zones.available.names
  private_subnets      = ["192.168.96.0/19", "192.168.128.0/19", "192.168.160.0/19"]
  public_subnets       = ["192.168.0.0/19", "192.168.32.0/19", "192.168.64.0/19"]
  enable_nat_gateway   = true
  single_nat_gateway   = true
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
  myip = "203.141.59.1"
  myip_cidr = "${local.myip}/32"
  podinfo_nodeport = 30080
}

resource "aws_security_group" "allow_http_from_me" {
  name        = "allow_http_from_me"
  description = "Allow inbound traffic"
  vpc_id      = "${module.vpc.vpc_id}"

  ingress {
    description = "HTTTP from me"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = [local.myip_cidr]
  }

  tags = {
    Name = "allow_http_from_me"
  }
}

resource "aws_security_group" "public_alb" {
  name        = "public_alb"
  description = "Allow healthcheck against private and public nodes"
  vpc_id      = "${module.vpc.vpc_id}"

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = [module.vpc.vpc_cidr_block]
  }

  tags = {
    Name = "public_alb"
  }
}

resource "aws_security_group" "public_alb_private_backend" {
  name        = "public_alb_private_backend"
  description = "Allow healthcheck against nodes"
  vpc_id      = "${module.vpc.vpc_id}"

  ingress {
    description = "Allow healthchecking and forwarding from alb"
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    security_groups = [aws_security_group.public_alb.id]
  }

  tags = {
    Name = "public_alb_private_backend"
  }
}

resource "aws_alb" "alb" {
  name                       = "existingvpc2"
  security_groups            = [aws_security_group.public_alb.id, aws_security_group.allow_http_from_me.id]
  subnets                    = module.vpc.public_subnets
  internal                   = false
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

resource "eksctl_cluster" "primary" {
  eksctl_bin = "eksctl-dev"
  name = "existingvpc2"
  region = "us-east-2"
  api_version = "eksctl.io/v1alpha5"
  version = "1.16"
  vpc_id = module.vpc.vpc_id
  revision = 3
  spec = <<EOS

nodeGroups:
  - name: ng2
    instanceType: m5.large
    desiredCapacity: 1
    securityGroups:
      attachIDs:
      - ${aws_security_group.public_alb_private_backend.id}

iam:
  withOIDC: true
  serviceAccounts: []

vpc:
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

  manifests = [
    <<EOS
apiVersion: apps/v1
kind: Deployment
metadata:
  name: podinfo
spec:
  minReadySeconds: 3
  revisionHistoryLimit: 5
  progressDeadlineSeconds: 60
  strategy:
    rollingUpdate:
      maxUnavailable: 0
    type: RollingUpdate
  selector:
    matchLabels:
      app: podinfo
  template:
    metadata:
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9797"
      labels:
        app: podinfo
    spec:
      containers:
      - name: podinfod
        image: stefanprodan/podinfo:4.0.1
        imagePullPolicy: IfNotPresent
        ports:
        - name: http
          containerPort: 9898
          protocol: TCP
        - name: http-metrics
          containerPort: 9797
          protocol: TCP
        - name: grpc
          containerPort: 9999
          protocol: TCP
        command:
        - ./podinfo
        - --port=9898
        - --port-metrics=9797
        - --grpc-port=9999
        - --grpc-service-name=podinfo
        - --level=info
        - --random-delay=false
        - --random-error=false
        env:
        - name: PODINFO_UI_COLOR
          value: "#34577c"
        livenessProbe:
          exec:
            command:
            - podcli
            - check
            - http
            - localhost:9898/healthz
          initialDelaySeconds: 5
          timeoutSeconds: 5
        readinessProbe:
          exec:
            command:
            - podcli
            - check
            - http
            - localhost:9898/readyz
          initialDelaySeconds: 5
          timeoutSeconds: 5
        resources:
          limits:
            cpu: 2000m
            memory: 512Mi
          requests:
            cpu: 100m
            memory: 64Mi
---
# k create service nodeport --tcp 80:9898 --node-port 30080 podinfo -o yaml --dry-run
apiVersion: v1
kind: Service
metadata:
  labels:
    app: podinfo
  name: podinfo
spec:
  ports:
  - name: 80-9898
    nodePort: 30080
    port: 80
    protocol: TCP
    targetPort: 9898
  selector:
    app: podinfo
  type: NodePort
EOS
    ,
    file("manifests/metrics-server-v0.3.6/all.yaml"),
  ]

  pods_readiness_check {
    namespace = "default"
    labels = {
      app = "podinfo"
    }
    timeout_sec = 300
  }

  kubernetes_resource_deletion_before_destroy {
    namespace = "flux"
    kind = "deployment"
    name = "flux"
  }

  alb_attachment {
    protocol = "http"

    node_port = 30080
    node_group_name = "ng2"
    weight = 19

    // We specify listener rather than alb, so that we can reuse any listener that is created out-of-band
    listener_arn = aws_alb_listener.podinfo.arn

    // alb_attachment manages only one alb listener rule
    //
    // this specifies the priority of the only rule
    priority = 10

    // Settings below are for configuring rule conditions
    hosts = ["example.com", "*.example.com"]
    methods = ["get"]
    path_patterns = ["/*"]
//    source_ips = ["1.2.3.4/32"]
  }

  depends_on = [module.vpc]
}
