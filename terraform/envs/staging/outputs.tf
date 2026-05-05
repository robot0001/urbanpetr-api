output "api_url" {
  value       = module.api.api_url
  description = "API Gateway execute-api URL for this PR staging environment."
}

output "migrations_lambda_name" {
  value       = module.api.migrations_lambda_name
  description = "Migrations Lambda function name — invoke after apply to run DB setup."
}

output "seed_lambda_name" {
  value       = module.api.seed_lambda_name
  description = "Seed Lambda function name — invoke after migrate to load test data."
}

output "custom_domain_url" {
  value       = "https://api-stage${var.pr_number}.urbanpetr.com"
  description = "Custom domain URL for this PR staging environment."
}

output "api_gateway_target_domain" {
  value       = module.api.api_gateway_target_domain
  description = "Target domain for the Route53 ALIAS record."
}

output "api_gateway_hosted_zone_id" {
  value       = module.api.api_gateway_hosted_zone_id
  description = "Hosted zone ID for the Route53 ALIAS record."
}
