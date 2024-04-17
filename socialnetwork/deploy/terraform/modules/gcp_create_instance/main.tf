terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 5.12"
    }
  }
}

resource "google_compute_instance" "default" {
  name            = var.instance_name
  machine_type    = "e2-medium"
  zone            = var.zone
  hostname        = var.hostname

  boot_disk {
    initialize_params {
      image = var.image
    }
  }

  network_interface {
    network = "default"
    access_config {
    }
  }

  service_account {
    # Google recommends custom service accounts that have cloud-platform scope and permissions granted via IAM Roles.
    scopes = ["cloud-platform"]
  }
}

