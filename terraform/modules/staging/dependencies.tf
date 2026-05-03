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
