variable "gcp_credentials_path" {
  description   = "The GCP credentials path"
  sensitive     = true
  type          = string
  default       = ".../gcp/credentials.json"
}

variable "gcp_project" {
  description   = "The GCP project name"
  sensitive     = true
  type          = string
}

variable "gcp_region" {
  description   = "The GCP region"
  sensitive     = true
  type          = string
}

variable "gcp_zone" {
  description   = "The GCP zone"
  sensitive     = true
  type          = string
}
