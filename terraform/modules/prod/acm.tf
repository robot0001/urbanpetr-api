# The eu-central-1 ACM certificate for the old API Gateway custom domain has been
# removed. CloudFront now terminates TLS — its us-east-1 cert is in cloudfront_api.tf.
# Terraform will destroy aws_acm_certificate.api and its validation resources from state.
