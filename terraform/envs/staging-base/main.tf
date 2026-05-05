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

data "terraform_remote_state" "platform" {
  backend = "s3"
  config = {
    bucket = "urbanpetr-tf-state-staging"
    key    = "platform/staging/terraform.tfstate"
    region = "eu-central-1"
  }
}

# Shared Lambda security group reused by all PR staging environments.
# Avoids the ~15 min ENI detach wait on terraform destroy per PR.
resource "aws_security_group" "staging_lambda" {
  name        = "urbanpetr-api-staging-lambda"
  description = "Shared Lambda SG for all PR staging environments"
  vpc_id      = data.terraform_remote_state.platform.outputs.vpc_id

  tags = {
    Project     = "urbanpetr-api"
    Environment = "staging"
  }
}

resource "aws_vpc_security_group_egress_rule" "staging_lambda_all" {
  security_group_id = aws_security_group.staging_lambda.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1"
}

resource "aws_vpc_security_group_ingress_rule" "staging_lambda_to_rds" {
  security_group_id            = data.terraform_remote_state.platform.outputs.rds_security_group_id
  referenced_security_group_id = aws_security_group.staging_lambda.id
  from_port                    = 5432
  to_port                      = 5432
  ip_protocol                  = "tcp"
  description                  = "urbanpetr-api staging Lambdas to RDS"
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

# Wildcard cert for api-stage{N}.urbanpetr.com custom domains on PR envs.
# After applying, add the CNAME from wildcard_cert_validation_cname output to
# Route53 in Account A (one-time). The cert auto-validates and stays ISSUED.
resource "aws_acm_certificate" "wildcard" {
  domain_name       = "*.urbanpetr.com"
  validation_method = "DNS"

  tags = {
    Project     = "urbanpetr-api"
    Environment = "staging"
  }

  lifecycle {
    create_before_destroy = true
  }
}

output "wildcard_cert_arn" {
  value       = aws_acm_certificate.wildcard.arn
  description = "Wildcard cert ARN in Account B — PENDING_VALIDATION until DNS record is added to Account A."
}

output "wildcard_cert_validation_cname" {
  value = {
    for dvo in aws_acm_certificate.wildcard.domain_validation_options : dvo.domain_name => {
      name  = dvo.resource_record_name
      type  = dvo.resource_record_type
      value = dvo.resource_record_value
    }
  }
  description = "Add this CNAME to Route53 in Account A to validate the cert (one-time)."
}
