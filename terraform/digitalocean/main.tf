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
      - cp .env.example .env
      - PUBLIC_IP=$(curl -s http://ifconfig.me)
      - sed -i "s|GITEA_PUBLIC_URL=.*|GITEA_PUBLIC_URL=http://$PUBLIC_IP:3222|" .env
      - |
        if [ -n "${var.domain}" ]; then
          sed -i "s|^DOMAIN=.*|DOMAIN=${var.domain}|" .env
          sed -i "s|^GITEA_PUBLIC_URL=.*|GITEA_PUBLIC_URL=http://${var.domain}:3222|" .env
        fi
      - docker compose up -d --build
  EOF
}

data "digitalocean_ssh_key" "user" {
  count = var.ssh_key_name != "" ? 1 : 0
  name  = var.ssh_key_name
}

resource "digitalocean_droplet" "this" {
  image      = "ubuntu-24-04-x64"
  name       = "ft-hackthon"
  region     = var.region
  size       = var.size
  ssh_keys   = var.ssh_key_name != "" ? [data.digitalocean_ssh_key.user[0].id] : []
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
