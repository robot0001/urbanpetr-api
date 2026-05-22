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

output "waf_arn" {
  value       = aws_wafv2_web_acl.shared.arn
  description = "Shared CLOUDFRONT-scope WAF WebACL ARN — consumed by urbanpetr.com to protect the frontend CloudFront distribution."
}

output "kill_switch_ip_set_id" {
  value       = aws_wafv2_ip_set.kill_switch.id
  description = "IPv4 kill-switch IP set ID — passed to the kill-switch Lambda."
}

output "kill_switch_ip_set_v6_id" {
  value       = aws_wafv2_ip_set.kill_switch_v6.id
  description = "IPv6 kill-switch IP set ID — passed to the kill-switch Lambda."
}

output "api_cloudfront_distribution_id" {
  value       = aws_cloudfront_distribution.api.id
  description = "API CloudFront distribution ID — passed to the kill-switch Lambda."
}

output "kill_switch_sns_topic_arn" {
  value       = aws_sns_topic.kill_switch.arn
  description = "SNS topic ARN that triggers the kill-switch Lambda."
}

