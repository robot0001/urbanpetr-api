locals {
  vpc_id                = data.terraform_remote_state.platform.outputs.vpc_id
  private_subnet_ids    = data.terraform_remote_state.platform.outputs.private_subnet_ids
  rds_security_group_id = data.terraform_remote_state.platform.outputs.rds_security_group_id
  rds_master_secret_arn = data.terraform_remote_state.platform.outputs.rds_master_secret_arn

  zone_id    = data.terraform_remote_state.foundation.outputs.prod_zone_id
  api_domain = "api.${var.domain_name}"

  common_tags = {
    Project     = var.project_name
    Environment = var.environment
  }
}
