output "api_endpoint" {
  value       = module.api.api_endpoint
  description = "Live API endpoint (custom domain)."
}

output "waf_arn" {
  value       = module.api.waf_arn
  description = "Shared CLOUDFRONT-scope WAF WebACL ARN — consumed by urbanpetr.com to protect the frontend CloudFront distribution."
}

output "cognito_user_pool_id" {
  value       = module.api.cognito_user_pool_id
  description = "Cognito User Pool ID — consumed by football-api to attach its own entity-manage groups to the shared pool."
}
