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
  name               = "${local.name_prefix}-lambda"
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
    resources = [data.aws_secretsmanager_secret.db_app.arn]
  }
}

resource "aws_iam_role_policy" "api_lambda" {
  name   = "app-secrets"
  role   = aws_iam_role.api_lambda.id
  policy = data.aws_iam_policy_document.api_lambda.json
}

# Migrations Lambda role — read/write all three staging secrets + read master
resource "aws_iam_role" "migrations_lambda" {
  name               = "${local.name_prefix}-migrations"
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
      data.aws_secretsmanager_secret.db_app.arn,
      data.aws_secretsmanager_secret.db_migrator.arn,
      data.aws_secretsmanager_secret.db_readonly.arn,
    ]
  }
}

resource "aws_iam_role_policy" "migrations_lambda" {
  name   = "migration-secrets"
  role   = aws_iam_role.migrations_lambda.id
  policy = data.aws_iam_policy_document.migrations_lambda.json
}

# Seed Lambda role — read app secret only (same as API role)
resource "aws_iam_role" "seed_lambda" {
  name               = "${local.name_prefix}-seed"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json
  tags               = local.common_tags
}

resource "aws_iam_role_policy_attachment" "seed_lambda_vpc" {
  role       = aws_iam_role.seed_lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaVPCAccessExecutionRole"
}

data "aws_iam_policy_document" "seed_lambda" {
  statement {
    sid       = "ReadAppSecret"
    effect    = "Allow"
    actions   = ["secretsmanager:GetSecretValue"]
    resources = [data.aws_secretsmanager_secret.db_app.arn]
  }
}

resource "aws_iam_role_policy" "seed_lambda" {
  name   = "app-secrets"
  role   = aws_iam_role.seed_lambda.id
  policy = data.aws_iam_policy_document.seed_lambda.json
}
