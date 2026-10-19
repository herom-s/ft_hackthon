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
      - sed -i "s|^DOMAIN=.*|DOMAIN=|" .env
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

resource "digitalocean_firewall" "this" {
  name = "ft-hackthon"

  droplet_ids = [digitalocean_droplet.this.id]

  inbound_rule {
    protocol         = "tcp"
    port_range       = "22"
    source_addresses = ["0.0.0.0/0"]
  }
  inbound_rule {
    protocol         = "tcp"
    port_range       = "80"
    source_addresses = ["0.0.0.0/0"]
  }
  inbound_rule {
    protocol         = "tcp"
    port_range       = "8443"
    source_addresses = ["0.0.0.0/0"]
  }
  inbound_rule {
    protocol         = "tcp"
    port_range       = "3222"
    source_addresses = ["0.0.0.0/0"]
  }
  outbound_rule {
    protocol              = "tcp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0"]
  }
  outbound_rule {
    protocol              = "udp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0"]
  }
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

resource "digitalocean_domain" "this" {
  count      = var.domain != "" ? 1 : 0
  name       = var.domain
  ip_address = digitalocean_droplet.this.ipv4_address
}

resource "digitalocean_record" "www" {
  count  = var.domain != "" ? 1 : 0
  domain = digitalocean_domain.this[0].id
  type   = "A"
  name   = "www"
  value  = digitalocean_droplet.this.ipv4_address
}

output "ip" {
  value = digitalocean_droplet.this.ipv4_address
}

output "ssh_command" {
  value = "ssh root@${digitalocean_droplet.this.ipv4_address}"
}

output "api_url" {
  value = var.domain != "" ? "https://${var.domain}:8443/api/v1" : "https://${digitalocean_droplet.this.ipv4_address}:8443/api/v1"
}
