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

resource "digitalocean_droplet" "this" {
  image      = "ubuntu-24-04-x64"
  name       = "ft-hackthon"
  region     = var.region
  size       = var.size
  user_data  = local.cloud_init
  monitoring = true
}

output "ip" {
  value = digitalocean_droplet.this.ipv4_address
}

output "ssh_command" {
  value = "ssh root@${digitalocean_droplet.this.ipv4_address}"
}

output "api_url" {
  value = "https://${digitalocean_droplet.this.ipv4_address}:8343/api/v1"
}
