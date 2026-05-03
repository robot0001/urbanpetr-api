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
