variable "project" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region"
  type        = string
  default     = "europe-west1"
}

variable "zone" {
  description = "GCP zone (defaults to region-b)"
  type        = string
  default     = ""
}

variable "size" {
  description = "Machine type (e2-small = 2 vCPU shared, 2 GB RAM)"
  type        = string
  default     = "e2-small"
}

variable "domain" {
  description = "Domain for Let's Encrypt HTTPS (set DOMAIN in .env)"
  type        = string
  default     = ""
}

variable "ssh_key_name" {
  description = "SSH username (set SSH_KEY_NAME in .env)"
  type        = string
  default     = ""
}

variable "ssh_public_key" {
  description = "SSH public key content (set SSH_PUBLIC_KEY_PATH in .env)"
  type        = string
  default     = ""
}

variable "repo_url" {
  description = "Git repository URL"
  type        = string
  default     = "https://github.com/herom-s/ft_hackthon.git"
}
