locals {
  vpc_id                = data.terraform_remote_state.platform.outputs.vpc_id
  private_subnet_ids    = data.terraform_remote_state.platform.outputs.private_subnet_ids
  rds_security_group_id = data.terraform_remote_state.platform.outputs.rds_security_group_id
  rds_master_secret_arn = data.terraform_remote_state.platform.outputs.rds_master_secret_arn

  name_prefix = "urbanpetr-api-staging-${var.pr_number}"
  db_name     = "urbanpetr_api_pr_${var.pr_number}"

  common_tags = {
    Project     = var.project_name
    Environment = var.environment
    PRNumber    = var.pr_number
  }
}
