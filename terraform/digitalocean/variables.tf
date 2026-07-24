variable "region" {
  description = "DigitalOcean region"
  type        = string
  default     = "fra1"
}

variable "size" {
  description = "Droplet size"
  type        = string
  default     = "s-2vcpu-4gb"
}

variable "repo_url" {
  description = "Git repository URL"
  type        = string
  default     = "https://github.com/herom-s/ft_hackthon.git"
}
