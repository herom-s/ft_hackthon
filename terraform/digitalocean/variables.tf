variable "region" {
  description = "DigitalOcean region (nyc3 is closest to Brazil)"
  type        = string
  default     = "nyc3"
}

variable "size" {
  description = "Droplet size (s-1vcpu-2gb = $12/mo, minimum for grading to work)"
  type        = string
  default     = "s-1vcpu-2gb"
}

variable "repo_url" {
  description = "Git repository URL"
  type        = string
  default     = "https://github.com/herom-s/ft_hackthon.git"
}
