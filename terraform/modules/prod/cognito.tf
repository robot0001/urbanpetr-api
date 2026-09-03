data "aws_secretsmanager_secret_version" "google_oauth" {
  secret_id = "urbanpetr/cognito/google_oauth"
}

locals {
  google_oauth = jsondecode(data.aws_secretsmanager_secret_version.google_oauth.secret_string)
}

resource "aws_cognito_user_pool" "main" {
  name = "urbanpetr-${var.environment}"

  # No self-registration — admin only.
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
  tags                     = local.common_tags
}

# Prefix domain: urbanpetr-prod.auth.eu-central-1.amazoncognito.com
resource "aws_cognito_user_pool_domain" "main" {
  domain       = "urbanpetr-${var.environment}"
  user_pool_id = aws_cognito_user_pool.main.id
}

resource "aws_cognito_user_group" "admin" {
  name         = "urbanpetr_admin"
  user_pool_id = aws_cognito_user_pool.main.id
  description  = "Full access to all API endpoints"
}

resource "aws_cognito_identity_provider" "google" {
  user_pool_id  = aws_cognito_user_pool.main.id
  provider_name = "Google"
  provider_type = "Google"

  provider_details = {
    client_id                     = local.google_oauth.client_id
    client_secret                 = local.google_oauth.client_secret
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

resource "aws_cognito_user_pool_client" "admin_spa" {
  name         = "admin-spa-${var.environment}"
  user_pool_id = aws_cognito_user_pool.main.id

  generate_secret = false

  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["email", "openid"]
  allowed_oauth_flows_user_pool_client = true

  supported_identity_providers = ["Google"]

  callback_urls = [
    "https://admin.urbanpetr.com/auth-callback",
    "https://football-admin.urbanpetr.com/auth-callback",
    "http://localhost:3001/auth-callback",
  ]
  logout_urls = [
    "https://admin.urbanpetr.com",
    "https://football-admin.urbanpetr.com",
    "http://localhost:3001",
  ]

  access_token_validity  = 60    # minutes
  refresh_token_validity = 43200 # minutes (30 days)

  token_validity_units {
    access_token  = "minutes"
    refresh_token = "minutes"
  }

  read_attributes  = ["email", "email_verified"]
  write_attributes = ["email"]
}
