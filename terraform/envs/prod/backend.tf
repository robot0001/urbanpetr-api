terraform {
  backend "s3" {
    bucket         = "urbanpetr-tf-state"
    key            = "urbanpetr_api/prod/terraform.tfstate"
    dynamodb_table = "terraform-locks"
    region         = "eu-central-1"
    encrypt        = true
  }
}
