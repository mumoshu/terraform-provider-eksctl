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
