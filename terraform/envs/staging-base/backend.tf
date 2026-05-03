terraform {
  backend "s3" {
    bucket         = "urbanpetr-tf-state-staging"
    key            = "urbanpetr_api/staging-base/terraform.tfstate"
    dynamodb_table = "terraform-locks"
    region         = "eu-central-1"
    encrypt        = true
  }
}
