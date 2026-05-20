terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }
}

provider "aws" {
  region = "eu-central-1"
}

module "api" {
  source = "../../modules/stable-staging"

  project_name = var.project_name
  environment  = var.environment
}
