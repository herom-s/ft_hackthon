variable "region" {
  description = "AWS region"
  type        = string
  default     = "eu-west-1"
}

variable "size" {
  description = "EC2 instance type (t3.small = 2 vCPU burstable, 2 GB RAM)"
  type        = string
  default     = "t3.small"
}

variable "domain" {
  description = "Domain for Let's Encrypt HTTPS (set DOMAIN in .env)"
  type        = string
  default     = ""
}

variable "ssh_key_name" {
  description = "AWS key pair name (set SSH_KEY_NAME in .env, e.g. my-keypair)"
  type        = string
  default     = ""
}

variable "repo_url" {
  description = "Git repository URL"
  type        = string
  default     = "https://github.com/herom-s/ft_hackthon.git"
}
