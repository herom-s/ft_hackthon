locals {
  cloud_init = <<-EOF
    #cloud-config
    package_update: true
    packages:
      - git
      - docker.io
      - docker-compose-v2
    runcmd:
      - systemctl enable --now docker
      - git clone ${var.repo_url} /opt/ft_hackthon
      - cd /opt/ft_hackthon
      - PUBLIC_IP=$(curl -s http://ifconfig.me)
      - sed -i "s|GITEA_PUBLIC_URL=.*|GITEA_PUBLIC_URL=http://$PUBLIC_IP:3222|" .env
      - docker compose up -d --build
  EOF
}

data "aws_ami" "ubuntu" {
  most_recent = true
  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd-gp3/ubuntu-noble-24.04-amd64-server-*"]
  }
  owners = ["099720109477"]
}

resource "aws_security_group" "this" {
  name        = "ft-hackthon"
  description = "ft_hackthon grading system"
}

resource "aws_vpc_security_group_ingress_rule" "main" {
  security_group_id = aws_security_group.this.id
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 8342
  to_port           = 8343
  ip_protocol       = "tcp"
}

resource "aws_vpc_security_group_ingress_rule" "gitea" {
  security_group_id = aws_security_group.this.id
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 3222
  to_port           = 3222
  ip_protocol       = "tcp"
}

resource "aws_vpc_security_group_ingress_rule" "ssh" {
  security_group_id = aws_security_group.this.id
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 22
  to_port           = 22
  ip_protocol       = "tcp"
}

resource "aws_vpc_security_group_egress_rule" "all" {
  security_group_id = aws_security_group.this.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
}

resource "aws_instance" "this" {
  ami                         = data.aws_ami.ubuntu.id
  instance_type               = var.size
  user_data                   = local.cloud_init
  user_data_replace_on_change = true
  vpc_security_group_ids      = [aws_security_group.this.id]
  associate_public_ip_address = true
  tags = { Name = "ft-hackthon" }
}

data "cloudflare_zone" "this" {
  count = var.domain != "" ? 1 : 0
  name  = join(".", slice(split(".", var.domain), 1, length(split(".", var.domain))))
}

resource "cloudflare_record" "this" {
  count   = var.domain != "" ? 1 : 0
  zone_id = data.cloudflare_zone.this[0].id
  name    = split(".", var.domain)[0]
  type    = "A"
  value   = aws_instance.this.public_ip
  proxied = false
}

output "ip" {
  value = aws_instance.this.public_ip
}

output "ssh_command" {
  value = "ssh ubuntu@${aws_instance.this.public_ip}"
}

output "api_url" {
  value = "https://${aws_instance.this.public_ip}:8343/api/v1"
}
