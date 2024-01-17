terraform {
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = ">= 5.12"
    }
  }
}

resource "google_compute_instance" "default" {
  name         = var.instance_name
  machine_type = "e2-medium"
  zone         = var.zone
  hostname     = var.hostname 

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

  metadata_startup_script = <<-EOF
    #!/bin/bash
    sudo apt update -y && sudo apt upgrade -y
    sudo apt install -y docker.io docker-compose dnsutils curl wget

    sleep 10
    gsutil cp -r gs://weaver-411410_cloudbuild/weaver-dsb-socialnetwork /home/mafaldacf/.
    sudo docker build -t mongodb-delayed:4.4.6 /home/mafaldacf/weaver-dsb-socialnetwork/docker/mongodb-delayed/.
    sudo docker build -t mongodb-setup:4.4.6 /home/mafaldacf/weaver-dsb-socialnetwork/docker/mongodb-setup/post-storage/.
    sudo docker build -t rabbitmq-setup:3.8 /home/mafaldacf/weaver-dsb-socialnetwork/docker/rabbitmq-setup/write-home-timeline/.
  EOF
}

