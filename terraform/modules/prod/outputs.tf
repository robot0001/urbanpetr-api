output "api_endpoint" {
  value       = "https://${local.api_domain}"
  description = "Live API endpoint."
}

output "api_gateway_log_group_arn" {
  value       = aws_cloudwatch_log_group.api_gateway.arn
  description = "ARN of the API Gateway access log group — passed to api_gateway module in PR 4."
}
