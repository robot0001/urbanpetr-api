module "api_gateway" {
  source = "github.com/robot0001/urbanpetr-foundation//modules/api_gateway_http?ref=v2.1.1"

  name                       = "urbanpetr-api-prod"
  lambda_invoke_arn          = module.api_lambda.invoke_arn
  lambda_function_name       = module.api_lambda.function_name
  access_log_destination_arn = aws_cloudwatch_log_group.api_gateway.arn
  cors_allow_origins         = ["https://urbanpetr.com", "https://admin.urbanpetr.com"]

  custom_tags = local.common_tags
}
