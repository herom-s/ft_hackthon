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
  description = "Machine type"
  type        = string
  default     = "e2-medium"
}

variable "repo_url" {
  description = "Git repository URL"
  type        = string
  default     = "https://github.com/herom-s/ft_hackthon.git"
}
