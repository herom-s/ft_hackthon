variable "floci_endpoint" {
  type    = string
  default = "http://localhost:14588"
}

# floci-gcp has no auto-mode VPCs (so no "default" network) and no public OS
# image catalog. These stand in for what a real project already has, and the
# override file points the module's firewall and instance at them.
resource "google_compute_network" "floci" {
  name                    = "ft-hackthon-test"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "floci" {
  name          = "ft-hackthon-test"
  network       = google_compute_network.floci.id
  region        = var.region
  ip_cidr_range = "10.10.0.0/24"
}

resource "google_compute_disk" "floci_image_source" {
  name = "ft-hackthon-ubuntu-source"
  zone = local.zone
  size = 10
}

resource "google_compute_image" "floci_ubuntu" {
  name        = "ubuntu-2404-lts-amd64"
  source_disk = google_compute_disk.floci_image_source.id
}
