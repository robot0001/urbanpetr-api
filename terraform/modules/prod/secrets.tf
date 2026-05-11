# Placeholder secrets — provision.go fills in the values on first migrate run.
# YouTube Data API v3 key — set the value manually in the AWS console after apply.

resource "aws_secretsmanager_secret" "youtube_api_key" {
  name                    = "urbanpetr/api/youtube_api_key"
  recovery_window_in_days = 0

  tags = local.common_tags
}

resource "aws_secretsmanager_secret" "db_app" {
  name                    = "urbanpetr/api/db/app"
  recovery_window_in_days = 0

  tags = local.common_tags
}

resource "aws_secretsmanager_secret" "db_migrator" {
  name                    = "urbanpetr/api/db/migrator"
  recovery_window_in_days = 0

  tags = local.common_tags
}

resource "aws_secretsmanager_secret" "db_readonly" {
  name                    = "urbanpetr/rds/readonly"
  recovery_window_in_days = 0

  tags = local.common_tags
}
