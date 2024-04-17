provider "google" {
  credentials = file(var.gcp_credentials_path)
  project = var.gcp_project
  region  = var.gcp_region
  zone    = var.gcp_zone
}

module "gcp_create_storage_instance_manager" {
  source        = "./modules/gcp_create_instance"
  instance_name = "weaver-dsb-db-manager"
  hostname      = "weaver-dsb-db-manager.prod"
  zone          = "europe-west3-a"
  image         = "debian-cloud/debian-11"
  providers = {
    google = google
  }
}

module "gcp_create_storage_instance_eu" {
  source        = "./modules/gcp_create_instance"
  instance_name = "weaver-dsb-db-eu"
  hostname      = "weaver-dsb-db-eu.prod"
  zone          = "europe-west3-a"
  image         = "debian-cloud/debian-11"
  providers = {
    google = google
  }
}

module "gcp_create_storage_instance-us" {
  source        = "./modules/gcp_create_instance"
  instance_name = "weaver-dsb-db-us"
  hostname      = "weaver-dsb-db-us.prod"
  zone          = "us-central1-a"
  image         = "debian-cloud/debian-11"
  providers = {
    google = google
  }
}

module "gcp_create_app_instance-eu" {
  source        = "./modules/gcp_create_instance"
  instance_name = "weaver-dsb-app-eu"
  hostname      = "weaver-dsb-app-eu.prod"
  zone          = "europe-west3-a"
  image         = "debian-cloud/debian-11"
  providers = {
    google = google
  }
}

module "gcp_create_app_instance-us" {
  source        = "./modules/gcp_create_instance"
  instance_name = "weaver-dsb-app-us"
  hostname      = "weaver-dsb-app-us.prod"
  zone          = "us-central1-a"
  image         = "debian-cloud/debian-11"
  providers = {
    google = google
  }
}

module "gcp_create_app_instance-wrk2" {
  source        = "./modules/gcp_create_instance"
  instance_name = "weaver-dsb-app-wrk2"
  hostname      = "weaver-dsb-app-wrk2.prod"
  zone          = "europe-west3-a"
  image         = "debian-cloud/debian-11"
  providers = {
    google = google
  }
}
