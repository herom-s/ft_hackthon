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

variable "domain" {
  description = "Optional domain name (creates Cloudflare A record)"
  type        = string
  default     = ""
}

variable "repo_url" {
  description = "Git repository URL"
  type        = string
  default     = "https://github.com/herom-s/ft_hackthon.git"
}
