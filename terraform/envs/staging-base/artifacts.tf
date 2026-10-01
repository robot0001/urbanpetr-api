# urbanpetr-artifacts-staging predates Terraform here and is shared by the PR
# environments of football-api, urbanpetr-api and urbanpetr-monitoring, so
# only its lifecycle is managed, not the bucket. Same rules as the prod
# bucket — see terraform/modules/prod/artifacts.tf for why each one exists.
resource "aws_s3_bucket_lifecycle_configuration" "artifacts_staging" {
  bucket = "urbanpetr-artifacts-staging"

  rule {
    id     = "expire-old-zips"
    status = "Enabled"
    filter {}
    noncurrent_version_expiration {
      noncurrent_days = 30
    }
    expiration {
      expired_object_delete_marker = true
    }
  }

  rule {
    id     = "expire-current-zips"
    status = "Enabled"
    filter {
      object_size_greater_than = 1024
    }
    expiration {
      days = 30
    }
  }
}
