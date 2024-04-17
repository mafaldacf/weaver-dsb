variable "image" {
  description   = "The image to create the instance"
  type          = string
}

variable "instance_name" {
  description   = "The instance name"
  type          = string
}

variable "hostname" {
  description   = "The hostname that represents the instance"
  type          = string
}

variable "zone" {
  description   = "The zone where the instance is being created"
  type          = string
}
