terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }
}

provider "aws" {
  region = "eu-central-1"
}

# Placeholder secrets shared across all PR staging environments.
# provision.go fills in the values on first migrate run.

resource "aws_secretsmanager_secret" "db_app" {
  name                    = "urbanpetr/api/staging/db/app"
  recovery_window_in_days = 0

  tags = {
    Project     = "urbanpetr-api"
    Environment = "staging"
  }
}

resource "aws_secretsmanager_secret" "db_migrator" {
  name                    = "urbanpetr/api/staging/db/migrator"
  recovery_window_in_days = 0

  tags = {
    Project     = "urbanpetr-api"
    Environment = "staging"
  }
}

resource "aws_secretsmanager_secret" "db_readonly" {
  name                    = "urbanpetr/staging/rds/readonly"
  recovery_window_in_days = 0

  tags = {
    Project     = "urbanpetr-api"
    Environment = "staging"
  }
}
