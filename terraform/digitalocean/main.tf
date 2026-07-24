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
  image      = "ubuntu-24-04"
  name       = "ft-hackthon"
  region     = var.region
  size       = var.size
  user_data  = local.cloud_init
  monitoring = true
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
  value   = digitalocean_droplet.this.ipv4_address
  proxied = false
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
