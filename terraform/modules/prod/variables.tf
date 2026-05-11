variable "environment" {
  type = string
}

variable "project_name" {
  type = string
}

variable "domain_name" {
  type = string
}

variable "google_oauth_client_id" {
  type        = string
  description = "Google OAuth client ID for Cognito identity provider. Set via TF_VAR_google_oauth_client_id."
  default     = ""
}

variable "google_oauth_client_secret" {
  type        = string
  sensitive   = true
  description = "Google OAuth client secret for Cognito identity provider. Set via TF_VAR_google_oauth_client_secret."
  default     = ""
}
