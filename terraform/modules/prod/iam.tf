data "aws_iam_policy_document" "lambda_assume_role" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

# API Lambda role — read app secret only
resource "aws_iam_role" "api_lambda" {
  name               = "urbanpetr-api-prod-lambda"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json
  tags               = local.common_tags
}

resource "aws_iam_role_policy_attachment" "api_lambda_vpc" {
  role       = aws_iam_role.api_lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaVPCAccessExecutionRole"
}

data "aws_iam_policy_document" "api_lambda" {
  statement {
    sid       = "ReadAppSecret"
    effect    = "Allow"
    actions   = ["secretsmanager:GetSecretValue"]
    resources = [aws_secretsmanager_secret.db_app.arn]
  }

  statement {
    sid       = "ReadYouTubeAPIKey"
    effect    = "Allow"
    actions   = ["secretsmanager:GetSecretValue"]
    resources = [aws_secretsmanager_secret.youtube_api_key.arn]
  }
}

resource "aws_iam_role_policy" "api_lambda" {
  name   = "app-secrets"
  role   = aws_iam_role.api_lambda.id
  policy = data.aws_iam_policy_document.api_lambda.json
}

# Migrations Lambda role — read/write all four secrets
resource "aws_iam_role" "migrations_lambda" {
  name               = "urbanpetr-api-prod-migrations"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json
  tags               = local.common_tags
}

resource "aws_iam_role_policy_attachment" "migrations_lambda_vpc" {
  role       = aws_iam_role.migrations_lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaVPCAccessExecutionRole"
}

data "aws_iam_policy_document" "migrations_lambda" {
  statement {
    sid       = "ReadMasterSecret"
    effect    = "Allow"
    actions   = ["secretsmanager:GetSecretValue"]
    resources = [local.rds_master_secret_arn]
  }

  statement {
    sid    = "ProvisionSecrets"
    effect = "Allow"
    actions = [
      "secretsmanager:GetSecretValue",
      "secretsmanager:PutSecretValue",
    ]
    resources = [
      aws_secretsmanager_secret.db_app.arn,
      aws_secretsmanager_secret.db_migrator.arn,
      aws_secretsmanager_secret.db_readonly.arn,
    ]
  }
}

resource "aws_iam_role_policy" "migrations_lambda" {
  name   = "migration-secrets"
  role   = aws_iam_role.migrations_lambda.id
  policy = data.aws_iam_policy_document.migrations_lambda.json
}
