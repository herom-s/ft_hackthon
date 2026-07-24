variable "region" {
  description = "Azure region"
  type        = string
  default     = "westeurope"
}

variable "size" {
  description = "VM size"
  type        = string
  default     = "Standard_B2s"
}

variable "repo_url" {
  description = "Git repository URL"
  type        = string
  default     = "https://github.com/herom-s/ft_hackthon.git"
}
