variable "region" {
  description = "Azure region"
  type        = string
  default     = "northcentralus"
}

variable "size" {
  description = "VM size (Standard_B2als_v2 = 2 vCPU, 4 GB RAM in northcentralus)"
  type        = string
  default     = "Standard_B2als_v2"
}

variable "repo_url" {
  description = "Git repository URL"
  type        = string
  default     = "https://github.com/herom-s/ft_hackthon.git"
}

variable "domain" {
  description = "Your domain (enables Let's Encrypt + removes need for --insecure)"
  type        = string
  default     = ""
}

variable "ssh_public_key" {
  description = "Your public SSH key content (required for Azure)"
  type        = string
  default     = ""
}