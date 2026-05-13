variable "google_oauth_client_id" {
  type        = string
  description = "Google OAuth client ID. Set via TF_VAR_google_oauth_client_id."
  default     = ""
}

variable "google_oauth_client_secret" {
  type        = string
  sensitive   = true
  description = "Google OAuth client secret. Set via TF_VAR_google_oauth_client_secret."
  default     = ""
}

resource "aws_cognito_user_pool" "staging" {
  name = "urbanpetr-staging"

  admin_create_user_config {
    allow_admin_create_user_only = true
  }

  schema {
    name                = "email"
    attribute_data_type = "String"
    required            = true
    mutable             = true
  }

  auto_verified_attributes = ["email"]

  tags = {
    Project     = "urbanpetr-api"
    Environment = "staging"
  }
}

# Prefix domain: urbanpetr-staging.auth.eu-central-1.amazoncognito.com
resource "aws_cognito_user_pool_domain" "staging" {
  domain       = "urbanpetr-staging"
  user_pool_id = aws_cognito_user_pool.staging.id
}

resource "aws_cognito_user_group" "admin" {
  name         = "urbanpetr_admin"
  user_pool_id = aws_cognito_user_pool.staging.id
  description  = "Full access to all API endpoints"
}

resource "aws_cognito_identity_provider" "google" {
  count = var.google_oauth_client_id != "" ? 1 : 0

  user_pool_id  = aws_cognito_user_pool.staging.id
  provider_name = "Google"
  provider_type = "Google"

  provider_details = {
    client_id                     = var.google_oauth_client_id
    client_secret                 = var.google_oauth_client_secret
    authorize_scopes              = "email openid"
    attributes_url                = "https://people.googleapis.com/v1/people/me?personFields="
    attributes_url_add_attributes = "true"
    authorize_url                 = "https://accounts.google.com/o/oauth2/v2/auth"
    token_request_method          = "POST"
    token_url                     = "https://oauth2.googleapis.com/token"
    oidc_issuer                   = "https://accounts.google.com"
  }

  attribute_mapping = {
    email          = "email"
    username       = "sub"
    email_verified = "email_verified"
  }
}

# SPA client for local development (localhost:3001).
resource "aws_cognito_user_pool_client" "admin_spa" {
  name         = "admin-spa-staging"
  user_pool_id = aws_cognito_user_pool.staging.id

  generate_secret = false

  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["email", "openid"]
  allowed_oauth_flows_user_pool_client = true

  supported_identity_providers = length(aws_cognito_identity_provider.google) > 0 ? ["Google"] : ["COGNITO"]

  callback_urls = ["http://localhost:3001/auth-callback"]
  logout_urls   = ["http://localhost:3001"]

  access_token_validity  = 60
  refresh_token_validity = 43200

  token_validity_units {
    access_token  = "minutes"
    refresh_token = "minutes"
  }

  read_attributes  = ["email", "email_verified"]
  write_attributes = ["email"]
}

output "cognito_user_pool_id" {
  value       = aws_cognito_user_pool.staging.id
  description = "Staging Cognito User Pool ID — use as COGNITO_USER_POOL_ID in local .env for per-PR API and local admin dev."
}

output "cognito_client_id" {
  value       = aws_cognito_user_pool_client.admin_spa.id
  description = "Staging SPA client ID — use as NUXT_PUBLIC_COGNITO_CLIENT_ID in local .env."
}

output "cognito_hosted_ui_domain" {
  value       = "${aws_cognito_user_pool_domain.staging.domain}.auth.eu-central-1.amazoncognito.com"
  description = "Staging hosted UI domain — use as NUXT_PUBLIC_COGNITO_DOMAIN in local .env."
}
