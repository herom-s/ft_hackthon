variable "region" {
  description = "DigitalOcean region (nyc3 is closest to Brazil)"
  type        = string
  default     = "nyc3"
}

variable "size" {
  description = "Droplet size (s-2vcpu-4gb = $24/mo, enough for ~20 users)"
  type        = string
  default     = "s-2vcpu-4gb"
}

variable "repo_url" {
  description = "Git repository URL"
  type        = string
  default     = "https://github.com/herom-s/ft_hackthon.git"
}
