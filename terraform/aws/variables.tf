variable "region" {
  description = "AWS region"
  type        = string
  default     = "eu-west-1"
}

variable "size" {
  description = "EC2 instance type"
  type        = string
  default     = "t3.medium"
}

variable "repo_url" {
  description = "Git repository URL"
  type        = string
  default     = "https://github.com/herom-s/ft_hackthon.git"
}
