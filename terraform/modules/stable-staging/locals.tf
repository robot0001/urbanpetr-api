locals {
  vpc_id                = data.terraform_remote_state.platform.outputs.vpc_id
  private_subnet_ids    = data.terraform_remote_state.platform.outputs.private_subnet_ids
  rds_master_secret_arn = data.terraform_remote_state.platform.outputs.rds_master_secret_arn
  rds_host              = data.terraform_remote_state.platform.outputs.rds_endpoint
  rds_port              = data.terraform_remote_state.platform.outputs.rds_port

  name_prefix = "urbanpetr-api-staging"
  db_name     = "urbanpetr_api_staging"

  common_tags = {
    Project     = var.project_name
    Environment = var.environment
  }
}
