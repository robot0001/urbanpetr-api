output "api_endpoint" {
  value       = "https://${local.api_domain}"
  description = "Live API endpoint."
}

output "api_gateway_log_group_arn" {
  value       = aws_cloudwatch_log_group.api_gateway.arn
  description = "ARN of the API Gateway access log group — passed to api_gateway module in PR 4."
}

output "cognito_user_pool_id" {
  value       = aws_cognito_user_pool.main.id
  description = "Cognito User Pool ID — set as COGNITO_USER_POOL_ID on the API Lambda (done by Terraform) and used for local dev."
}

output "cognito_client_id" {
  value       = aws_cognito_user_pool_client.admin_spa.id
  description = "SPA App Client ID — set as NUXT_PUBLIC_COGNITO_CLIENT_ID in GitHub Actions and local .env."
}

output "cognito_hosted_ui_domain" {
  value       = "${aws_cognito_user_pool_domain.main.domain}.auth.eu-central-1.amazoncognito.com"
  description = "Hosted UI domain — set as NUXT_PUBLIC_COGNITO_DOMAIN in GitHub Actions and local .env."
}

output "eic_endpoint_id" {
  value       = aws_ec2_instance_connect_endpoint.main.id
  description = "EC2 Instance Connect Endpoint ID — used by db-tunnel.sh to open a local tunnel to RDS."
}
