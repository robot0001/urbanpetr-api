# Placeholder secrets — provision.go fills in the values on first migrate run.

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
