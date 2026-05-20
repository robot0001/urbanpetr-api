output "api_endpoint" {
  value       = "https://api-staging.urbanpetr.com"
  description = "Stable staging API endpoint."
}

output "api_lambda_name" {
  value       = module.api.api_lambda_name
  description = "API Lambda function name."
}

output "migrations_lambda_name" {
  value       = module.api.migrations_lambda_name
  description = "Migrations Lambda function name."
}

output "api_gateway_target_domain" {
  value       = module.api.api_gateway_target_domain
  description = "Target domain for the Route53 ALIAS record in Account A."
}

output "api_gateway_hosted_zone_id" {
  value       = module.api.api_gateway_hosted_zone_id
  description = "Hosted zone ID for the Route53 ALIAS record."
}
