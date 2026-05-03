# Allow both Lambda functions to reach RDS on port 5432.
resource "aws_vpc_security_group_ingress_rule" "api_lambda_to_rds" {
  security_group_id            = local.rds_security_group_id
  referenced_security_group_id = module.api_lambda.security_group_id
  from_port                    = 5432
  to_port                      = 5432
  ip_protocol                  = "tcp"
  description                  = "urbanpetr-api Lambda → RDS"

  tags = local.common_tags
}

resource "aws_vpc_security_group_ingress_rule" "migrations_lambda_to_rds" {
  security_group_id            = local.rds_security_group_id
  referenced_security_group_id = module.migrations_lambda.security_group_id
  from_port                    = 5432
  to_port                      = 5432
  ip_protocol                  = "tcp"
  description                  = "urbanpetr-api migrations Lambda → RDS"

  tags = local.common_tags
}
