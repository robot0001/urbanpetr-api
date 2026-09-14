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

# One alarm per public API Lambda; any of them fires the kill switch, which
# takes down everything (both APIs, all CloudFront distributions, shared WAF).
#
# Thresholds sit ~2x above the highest legitimate minute seen in 2026-09
# (urbanpetr-api 547/min, football-api 719/min from bot runs), and must hold
# for 2 of 3 minutes so one bursty minute can't kill prod.
#
# The 2-of-3 window also covers late-arriving Lambda metrics: the previous
# single-period alarm (500/min, 1 of 1) never left OK even when urbanpetr-api
# hit 547/min on 2026-09-03.
locals {
  kill_switch_invocation_alarms = {
    urbanpetr-api = { function_name = "urbanpetr-api-prod", threshold = 1000 }
    football-api  = { function_name = "football-api-prod", threshold = 1500 }
  }
}

resource "aws_cloudwatch_metric_alarm" "api_invocations_spike" {
  for_each = local.kill_switch_invocation_alarms

  alarm_name          = "${each.key}-invocations-spike"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 3
  datapoints_to_alarm = 2
  metric_name         = "Invocations"
  namespace           = "AWS/Lambda"
  period              = 60
  statistic           = "Sum"
  threshold           = each.value.threshold
  treat_missing_data  = "notBreaching"

  dimensions = {
    FunctionName = each.value.function_name
  }

  alarm_actions = [aws_sns_topic.kill_switch.arn]

  tags = local.common_tags
}

# Keeps the existing urbanpetr-api alarm in place instead of a destroy + create
# of the same alarm name, which can race and leave no alarm at all.
moved {
  from = aws_cloudwatch_metric_alarm.api_invocations_spike
  to   = aws_cloudwatch_metric_alarm.api_invocations_spike["urbanpetr-api"]
}
