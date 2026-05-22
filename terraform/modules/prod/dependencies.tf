data "terraform_remote_state" "platform" {
  backend = "s3"
  config = {
    bucket = "urbanpetr-tf-state"
    key    = "platform/prod/terraform.tfstate"
    region = "eu-central-1"
  }
}

data "terraform_remote_state" "foundation" {
  backend = "s3"
  config = {
    bucket = "urbanpetr-tf-state"
    key    = "foundation/prod/terraform.tfstate"
    region = "eu-central-1"
  }
}

data "terraform_remote_state" "website" {
  backend = "s3"
  config = {
    bucket = "urbanpetr-tf-state"
    key    = "urbanpetr_website/prod/terraform.tfstate"
    region = "eu-central-1"
  }
}

data "terraform_remote_state" "admin" {
  backend = "s3"
  config = {
    bucket = "urbanpetr-tf-state"
    key    = "urbanpetr_admin/prod/terraform.tfstate"
    region = "eu-central-1"
  }
}
