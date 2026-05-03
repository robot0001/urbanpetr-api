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
