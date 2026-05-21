terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = ">= 3.0"
    }
  }
}

provider "aws" {
  region = "eu-central-1"
}

provider "aws" {
  alias  = "us_east_1"
  region = "us-east-1"
}

provider "random" {}

module "api" {
  source = "../../modules/prod"

  providers = {
    aws           = aws
    aws.us_east_1 = aws.us_east_1
    random        = random
  }

  environment  = var.environment
  project_name = var.project_name
  domain_name  = var.domain_name
}
