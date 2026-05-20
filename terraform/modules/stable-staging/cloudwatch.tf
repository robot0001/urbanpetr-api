resource "aws_cloudwatch_log_group" "api_lambda" {
  name              = "/aws/lambda/${local.name_prefix}"
  retention_in_days = 7
  tags              = local.common_tags
}

resource "aws_cloudwatch_log_group" "migrations_lambda" {
  name              = "/aws/lambda/${local.name_prefix}-migrations"
  retention_in_days = 7
  tags              = local.common_tags
}

resource "aws_cloudwatch_log_group" "api_gateway" {
  name              = "/aws/apigateway/${local.name_prefix}"
  retention_in_days = 7
  tags              = local.common_tags
}

data "aws_caller_identity" "current" {}

locals {
  loki_forwarder_arn = "arn:aws:lambda:eu-central-1:${data.aws_caller_identity.current.account_id}:function:urbanpetr-loki-forwarder-staging"
}

resource "aws_cloudwatch_log_subscription_filter" "api_lambda" {
  name            = "loki-forwarder"
  log_group_name  = aws_cloudwatch_log_group.api_lambda.name
  filter_pattern  = ""
  destination_arn = local.loki_forwarder_arn
}

resource "aws_cloudwatch_log_subscription_filter" "migrations_lambda" {
  name            = "loki-forwarder"
  log_group_name  = aws_cloudwatch_log_group.migrations_lambda.name
  filter_pattern  = ""
  destination_arn = local.loki_forwarder_arn
}

resource "aws_cloudwatch_log_subscription_filter" "api_gateway" {
  name            = "loki-forwarder"
  log_group_name  = aws_cloudwatch_log_group.api_gateway.name
  filter_pattern  = ""
  destination_arn = local.loki_forwarder_arn
}
