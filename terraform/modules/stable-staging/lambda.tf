module "api_lambda" {
  source = "github.com/robot0001/urbanpetr-foundation//modules/lambda_function?ref=v2.1.0"

  function_name      = local.name_prefix
  package_type       = "Zip"
  runtime            = "provided.al2023"
  handler            = "bootstrap"
  s3_bucket          = "urbanpetr-artifacts-staging"
  s3_key             = "urbanpetr-api/placeholder.zip"
  security_group_ids = [data.aws_security_group.staging_lambda.id]
  subnet_ids         = local.private_subnet_ids
  execution_role_arn = aws_iam_role.api_lambda.arn

  environment_variables = {
    APP_NAME                   = "urbanpetr-api"
    ENVIRONMENT                = var.environment
    DB_SECRET_ARN              = data.aws_secretsmanager_secret.db_app.arn
    DB_NAME                    = local.db_name
    DB_HOST                    = local.rds_host
    DB_PORT                    = local.rds_port
    LAMBDA_HANDLER_MODE        = "api"
    YOUTUBE_API_KEY_SECRET_ARN = data.aws_secretsmanager_secret.youtube_api_key.arn
    COGNITO_USER_POOL_ID       = tolist(data.aws_cognito_user_pools.staging.ids)[0]
    COGNITO_AWS_REGION         = "eu-central-1"
  }

  custom_tags = local.common_tags
}

module "migrations_lambda" {
  source = "github.com/robot0001/urbanpetr-foundation//modules/lambda_function?ref=v2.1.0"

  function_name      = "${local.name_prefix}-migrations"
  package_type       = "Zip"
  runtime            = "provided.al2023"
  handler            = "bootstrap"
  s3_bucket          = "urbanpetr-artifacts-staging"
  s3_key             = "urbanpetr-api/placeholder.zip"
  timeout            = 300
  security_group_ids = [data.aws_security_group.staging_lambda.id]
  subnet_ids         = local.private_subnet_ids
  execution_role_arn = aws_iam_role.migrations_lambda.arn

  environment_variables = {
    DB_MASTER_SECRET_ARN   = local.rds_master_secret_arn
    DB_MIGRATOR_SECRET_ARN = data.aws_secretsmanager_secret.db_migrator.arn
    DB_SECRET_ARN          = data.aws_secretsmanager_secret.db_app.arn
    DB_READONLY_SECRET_ARN = data.aws_secretsmanager_secret.db_readonly.arn
    DB_NAME                = local.db_name
    DB_HOST                = local.rds_host
    DB_PORT                = local.rds_port
    LAMBDA_HANDLER_MODE    = "migrate"
  }

  custom_tags = local.common_tags
}
