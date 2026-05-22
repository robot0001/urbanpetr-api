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

# --- Kill-switch alarm ---

resource "aws_sns_topic" "kill_switch" {
  name = "urbanpetr-kill-switch"
  tags = local.common_tags
}

# Fires when Lambda invocations exceed 500/min for a single 60-second period.
# Normal peak is 50–200 req/min; 500 indicates a clear traffic spike.
# The kill-switch Lambda subscribes to this topic (wired in a later PR).
resource "aws_cloudwatch_metric_alarm" "api_invocations_spike" {
  alarm_name          = "urbanpetr-api-invocations-spike"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "Invocations"
  namespace           = "AWS/Lambda"
  period              = 60
  statistic           = "Sum"
  threshold           = 500
  treat_missing_data  = "notBreaching"

  dimensions = {
    FunctionName = "urbanpetr-api-prod"
  }

  alarm_actions = [aws_sns_topic.kill_switch.arn]

  tags = local.common_tags
}
