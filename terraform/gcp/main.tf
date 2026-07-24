locals {
  zone = var.zone != "" ? var.zone : "${var.region}-b"

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

resource "google_compute_firewall" "this" {
  name    = "ft-hackthon"
  network = "default"
  allow {
    protocol = "tcp"
    ports    = ["22", "8342", "8343", "3222"]
  }
  source_ranges = ["0.0.0.0/0"]
}

resource "google_compute_instance" "this" {
  name         = "ft-hackthon"
  machine_type = var.size
  zone         = local.zone

  boot_disk {
    initialize_params { image = "ubuntu-os-cloud/ubuntu-2404-lts-amd64" }
  }

  network_interface {
    network = "default"
    access_config { network_tier = "STANDARD" }
  }

  metadata = {
    user-data = local.cloud_init
  }
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
  value   = google_compute_instance.this.network_interface[0].access_config[0].nat_ip
  proxied = false
}

output "ip" {
  value = google_compute_instance.this.network_interface[0].access_config[0].nat_ip
}

output "ssh_command" {
  value = "ssh ${google_compute_instance.this.name}@${google_compute_instance.this.network_interface[0].access_config[0].nat_ip}"
}

output "api_url" {
  value = "https://${google_compute_instance.this.network_interface[0].access_config[0].nat_ip}:8343/api/v1"
}
