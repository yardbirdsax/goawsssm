terraform {
  required_version = "1.0.8"
  required_providers {
    aws = {
      version = "~>3.54"
      source  = "hashicorp/aws"
    }
  }
}

provider "aws" {
  region = var.region
}

data "aws_ami" "ubuntu" {
  filter {
    name = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-bionic-18.04-amd64-server-*"]
  }
  most_recent = true
  owners = ["099720109477"]
}

data "aws_iam_policy_document" "assume_role" {
  statement {
    actions = [ "sts:AssumeRole" ]
    effect = "Allow"
    principals {
      type = "Service"
      identifiers = [
        "ec2.amazonaws.com"
      ]
    }  
  }
}

data "template_cloudinit_config" "cloudinit" {
  part {
    content = <<-EOT
    #!/bin/bash
    apt-get update
    apt-get install -y nginx
    echo hello world > /usr/share/nginx/html/index.html
    systemctl enable nginx
    systemctl start nginx
    EOT
  }
}

resource "aws_iam_role" "role" {
  assume_role_policy = data.aws_iam_policy_document.assume_role.json
  name = var.service
}

resource "aws_iam_role_policy_attachment" "ssm_policy" {
  role = aws_iam_role.role.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_instance_profile" "instance_profile" {
  role = aws_iam_role.role.name
  name = var.service
}

resource "aws_instance" "instance" {
  ami = data.aws_ami.ubuntu.id
  associate_public_ip_address = false
  iam_instance_profile = aws_iam_instance_profile.instance_profile.name
  instance_type = "t3.micro"
  user_data_base64 = data.template_cloudinit_config.cloudinit.rendered
}