data "terraform_remote_state" "platform" {
  backend = "s3"
  config = {
    bucket = "urbanpetr-tf-state-staging"
    key    = "platform/staging/terraform.tfstate"
    region = "eu-central-1"
  }
}

# Shared staging secrets — created once by terraform/envs/staging-base.
data "aws_secretsmanager_secret" "db_app" {
  name = "urbanpetr/api/staging/db/app"
}

data "aws_secretsmanager_secret" "db_migrator" {
  name = "urbanpetr/api/staging/db/migrator"
}

data "aws_secretsmanager_secret" "db_readonly" {
  name = "urbanpetr/staging/rds/readonly"
}

data "aws_acm_certificate" "wildcard" {
  domain      = "*.urbanpetr.com"
  statuses    = ["ISSUED"]
  most_recent = true
}

data "aws_secretsmanager_secret" "youtube_api_key" {
  name = "urbanpetr/api/staging/youtube_api_key"
}

data "aws_security_group" "staging_lambda" {
  name   = "urbanpetr-api-staging-lambda"
  vpc_id = local.vpc_id
}
