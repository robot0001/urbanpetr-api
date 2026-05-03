variable "pr_number" {
  type        = string
  description = "GitHub PR number — used to name all per-PR resources."
}

variable "project_name" {
  type = string
}

variable "environment" {
  type    = string
  default = "staging"
}
