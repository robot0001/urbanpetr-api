output "api_url" {
  value       = module.api_gateway.api_endpoint
  description = "API Gateway execute-api URL for this PR staging environment."
}

output "api_lambda_name" {
  value       = module.api_lambda.function_name
  description = "API Lambda function name."
}

output "migrations_lambda_name" {
  value       = module.migrations_lambda.function_name
  description = "Migrations Lambda function name."
}

output "seed_lambda_name" {
  value       = module.seed_lambda.function_name
  description = "Seed Lambda function name."
}

output "custom_domain_name" {
  value       = "api-stage${var.pr_number}.urbanpetr.com"
  description = "Custom domain name for this PR staging environment."
}

output "api_gateway_target_domain" {
  value       = module.api_gateway.custom_domain_target
  description = "Target domain for Route53 ALIAS record (API GW regional endpoint)."
}

output "api_gateway_hosted_zone_id" {
  value       = module.api_gateway.custom_domain_hosted_zone_id
  description = "Hosted zone ID for the Route53 ALIAS record."
}

output "api_gateway_log_group_arn" {
  value       = aws_cloudwatch_log_group.api_gateway.arn
  description = "ARN of the API Gateway access log group — passed to api_gateway module in PR 4."
}
