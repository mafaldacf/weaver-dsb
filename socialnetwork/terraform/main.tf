provider "google" {
  credentials = file(var.gcp_credentials_path)
  project = var.gcp_project
  region  = var.gcp_region
  zone    = var.gcp_zone
}

module "gcp_create_instance_manager" {
  source        = "./modules/gcp_create_instance"
  instance_name = "weaver-dsb-db-manager"
  hostname      = "weaver-dsb-db-manager.prod"
  zone          = "europe-west3-a"
  image         = "debian-cloud/debian-11"
  providers = {
    google = google
  }
}

module "gcp_create_instance_eu" {
  source        = "./modules/gcp_create_instance"
  instance_name = "weaver-dsb-db-eu"
  hostname      = "weaver-dsb-db-eu.prod"
  zone          = "europe-west3-a"
  image         = "debian-cloud/debian-11"
  providers = {
    google = google
  }
}

module "gcp_create_instance-us" {
  source        = "./modules/gcp_create_instance"
  instance_name = "weaver-dsb-db-us"
  hostname      = "weaver-dsb-db-us.prod"
  zone          = "us-central1-a"
  image         = "debian-cloud/debian-11"
  providers = {
    google = google
  }
}
