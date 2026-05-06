resource "aws_cloudwatch_log_group" "api_lambda" {
  name              = "/aws/lambda/urbanpetr-api-prod"
  retention_in_days = 7
  tags              = local.common_tags
}

resource "aws_cloudwatch_log_group" "migrations_lambda" {
  name              = "/aws/lambda/urbanpetr-api-prod-migrations"
  retention_in_days = 7
  tags              = local.common_tags
}

resource "aws_cloudwatch_log_group" "api_gateway" {
  name              = "/aws/apigateway/urbanpetr-api-prod"
  retention_in_days = 7
  tags              = local.common_tags
}

# Allows API Gateway (via the log delivery service) to write access logs.
# This is an account-level policy — only one instance needed per account.
resource "aws_cloudwatch_log_resource_policy" "api_gateway_logging" {
  policy_name = "urbanpetr-api-gateway-logging"
  policy_document = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "delivery.logs.amazonaws.com" }
      Action = [
        "logs:CreateLogDelivery",
        "logs:PutLogEvents",
        "logs:GetLogDelivery",
        "logs:UpdateLogDelivery",
        "logs:DeleteLogDelivery",
        "logs:DescribeLogGroups",
        "logs:DescribeResourcePolicies",
      ]
      Resource = "*"
    }]
  })
}
