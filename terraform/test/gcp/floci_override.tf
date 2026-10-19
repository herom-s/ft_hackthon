provider "google" {
  compute_custom_endpoint = "${var.floci_endpoint}/compute/v1/"
}

resource "google_compute_firewall" "this" {
  network = google_compute_network.floci.name
}

resource "google_compute_instance" "this" {
  boot_disk {
    initialize_params { image = google_compute_image.floci_ubuntu.self_link }
  }

  network_interface {
    network    = google_compute_network.floci.id
    subnetwork = google_compute_subnetwork.floci.id
    access_config { network_tier = "STANDARD" }
  }
}
