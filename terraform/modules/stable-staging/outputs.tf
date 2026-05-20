output "api_lambda_name" {
  value       = module.api_lambda.function_name
  description = "API Lambda function name."
}

output "migrations_lambda_name" {
  value       = module.migrations_lambda.function_name
  description = "Migrations Lambda function name — invoke after apply to run DB setup."
}

output "api_gateway_target_domain" {
  value       = module.api_gateway.custom_domain_target
  description = "Target domain for the Route53 ALIAS record in Account A."
}

output "api_gateway_hosted_zone_id" {
  value       = module.api_gateway.custom_domain_hosted_zone_id
  description = "Hosted zone ID for the Route53 ALIAS record."
}
