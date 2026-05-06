# Lambda auto-creates log groups on first invocation.
# These import blocks adopt the existing groups into Terraform state.
# Safe to leave in place — idempotent once the resource is in state.

import {
  to = module.api.aws_cloudwatch_log_group.api_lambda
  id = "/aws/lambda/urbanpetr-api-prod"
}

import {
  to = module.api.aws_cloudwatch_log_group.migrations_lambda
  id = "/aws/lambda/urbanpetr-api-prod-migrations"
}
