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

# Google IdP — only created once google_oauth_client_id is provided.
# Pass credentials via TF_VAR_google_oauth_client_id / TF_VAR_google_oauth_client_secret
# or as GitHub Actions secrets before applying.
resource "aws_cognito_identity_provider" "google" {
  count = var.google_oauth_client_id != "" ? 1 : 0

  user_pool_id  = aws_cognito_user_pool.main.id
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

resource "aws_cognito_user_pool_client" "admin_spa" {
  name         = "admin-spa-${var.environment}"
  user_pool_id = aws_cognito_user_pool.main.id

  generate_secret = false

  allowed_oauth_flows                  = ["code"]
  allowed_oauth_scopes                 = ["email", "openid"]
  allowed_oauth_flows_user_pool_client = true

  # Google IdP wired in once available; falls back to Cognito-only before that.
  supported_identity_providers = length(aws_cognito_identity_provider.google) > 0 ? ["Google"] : ["COGNITO"]

  callback_urls = [
    "https://admin.urbanpetr.com/callback",
    "http://localhost:3001/callback",
  ]
  logout_urls = [
    "https://admin.urbanpetr.com",
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
