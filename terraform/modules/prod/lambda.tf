module "api_lambda" {
  source = "github.com/robot0001/urbanpetr-foundation//modules/lambda_function?ref=v2.1.1"

  function_name      = "urbanpetr-api-prod"
  package_type       = "Zip"
  runtime            = "provided.al2023"
  handler            = "bootstrap"
  s3_bucket          = aws_s3_bucket.artifacts.id
  s3_key             = "urbanpetr-api/placeholder.zip"
  vpc_id             = local.vpc_id
  subnet_ids         = local.private_subnet_ids
  execution_role_arn = aws_iam_role.api_lambda.arn

  environment_variables = {
    APP_NAME                   = "urbanpetr-api"
    ENVIRONMENT                = var.environment
    DB_SECRET_ARN              = aws_secretsmanager_secret.db_app.arn
    DB_HOST                    = local.rds_host
    DB_PORT                    = tostring(local.rds_port)
    LAMBDA_HANDLER_MODE        = "api"
    YOUTUBE_API_KEY_SECRET_ARN = aws_secretsmanager_secret.youtube_api_key.arn
    COGNITO_USER_POOL_ID       = aws_cognito_user_pool.main.id
    COGNITO_AWS_REGION         = "eu-central-1"
    ORIGIN_SECRET              = random_password.origin_secret.result
  }

  custom_tags = local.common_tags
}

module "migrations_lambda" {
  source = "github.com/robot0001/urbanpetr-foundation//modules/lambda_function?ref=v2.1.1"

  function_name      = "urbanpetr-api-prod-migrations"
  package_type       = "Zip"
  runtime            = "provided.al2023"
  handler            = "bootstrap"
  s3_bucket          = aws_s3_bucket.artifacts.id
  s3_key             = "urbanpetr-api/placeholder.zip"
  timeout            = 300
  vpc_id             = local.vpc_id
  subnet_ids         = local.private_subnet_ids
  execution_role_arn = aws_iam_role.migrations_lambda.arn

  environment_variables = {
    DB_MASTER_SECRET_ARN   = local.rds_master_secret_arn
    DB_MIGRATOR_SECRET_ARN = aws_secretsmanager_secret.db_migrator.arn
    DB_SECRET_ARN          = aws_secretsmanager_secret.db_app.arn
    DB_READONLY_SECRET_ARN = aws_secretsmanager_secret.db_readonly.arn
    DB_HOST                = local.rds_host
    DB_PORT                = tostring(local.rds_port)
    LAMBDA_HANDLER_MODE    = "migrate"
  }

  custom_tags = local.common_tags
}
