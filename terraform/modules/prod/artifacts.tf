resource "aws_s3_bucket" "artifacts" {
  bucket = "urbanpetr-artifacts"

  tags = local.common_tags
}

resource "aws_s3_bucket_versioning" "artifacts" {
  bucket = aws_s3_bucket.artifacts.id
  versioning_configuration {
    status = "Enabled"
  }
}

# Every deploy (football-api, urbanpetr-api, urbanpetr-monitoring) uploads a
# <sha>.zip and points its Lambda at it in the same run, and Lambda keeps its
# own copy of the code, so a zip is dead weight once that run is over. Without
# expiring current versions the bucket grew ~3.8 GB a week (one ~11 MB zip
# per football-api commit). An older build is rebuilt from git, never pulled
# back out of here.
#
# Expiring a current version only adds a delete marker in a versioned bucket;
# the zip itself goes 30 days later via the noncurrent rule, so a mistake has
# a month to be undone.
resource "aws_s3_bucket_lifecycle_configuration" "artifacts" {
  bucket = aws_s3_bucket.artifacts.id
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
  # The size floor keeps the */placeholder*.zip objects (127 bytes) that
  # Terraform creates every Lambda from: they are uploaded once and never
  # touched again, so an age rule alone would delete them. Real builds are
  # 4 MB and up.
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

resource "aws_s3_bucket_public_access_block" "artifacts" {
  bucket                  = aws_s3_bucket.artifacts.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}
