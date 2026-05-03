terraform {
  backend "s3" {
    bucket         = "urbanpetr-tf-state-staging"
    key            = "urbanpetr_api/staging/terraform.tfstate"
    dynamodb_table = "terraform-locks"
    region         = "eu-central-1"
    encrypt        = true
    # CI overrides key per PR:
    # terraform init -backend-config="key=urbanpetr_api/staging-${PR_NUMBER}/terraform.tfstate"
  }
}
