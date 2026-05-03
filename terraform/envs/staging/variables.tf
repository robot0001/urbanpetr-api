variable "pr_number" {
  type        = string
  description = "GitHub PR number. Passed via -var='pr_number=...' in CI."
}

variable "project_name" {
  type = string
}

variable "environment" {
  type    = string
  default = "staging"
}
