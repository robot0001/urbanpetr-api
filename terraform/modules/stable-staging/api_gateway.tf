module "api_gateway" {
  source = "github.com/robot0001/urbanpetr-foundation//modules/api_gateway_http?ref=v2.1.1"

  name                       = local.name_prefix
  lambda_invoke_arn          = module.api_lambda.invoke_arn
  lambda_function_name       = module.api_lambda.function_name
  cors_allow_origins         = ["*"]
  custom_domain_name         = "api-staging.urbanpetr.com"
  certificate_arn            = data.aws_acm_certificate.wildcard.arn
  access_log_destination_arn = aws_cloudwatch_log_group.api_gateway.arn

  custom_tags = local.common_tags
}
