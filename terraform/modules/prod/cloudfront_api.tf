locals {
  # Strip "https://" and trailing "/" to get bare origin hostname for CloudFront
  api_origin_domain = split("/", trimprefix(module.api_gateway.api_endpoint, "https://"))[0]
}

resource "aws_acm_certificate" "api_cf" {
  provider          = aws.us_east_1
  domain_name       = local.api_domain
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }

  tags = local.common_tags
}

resource "aws_route53_record" "api_cf_cert_validation" {
  for_each = {
    for dvo in aws_acm_certificate.api_cf.domain_validation_options : dvo.domain_name => {
      name   = dvo.resource_record_name
      type   = dvo.resource_record_type
      record = dvo.resource_record_value
    }
  }

  zone_id         = local.zone_id
  name            = each.value.name
  type            = each.value.type
  records         = [each.value.record]
  ttl             = 60
  allow_overwrite = true
}

resource "aws_acm_certificate_validation" "api_cf" {
  provider                = aws.us_east_1
  certificate_arn         = aws_acm_certificate.api_cf.arn
  validation_record_fqdns = [for r in aws_route53_record.api_cf_cert_validation : r.fqdn]
}

resource "aws_cloudfront_distribution" "api" {
  enabled         = true
  is_ipv6_enabled = true
  aliases         = [local.api_domain]
  web_acl_id      = aws_wafv2_web_acl.shared.arn
  price_class     = "PriceClass_100"

  origin {
    domain_name = local.api_origin_domain
    origin_id   = "APIGatewayOrigin"

    custom_header {
      name  = "X-Origin-Secret"
      value = random_password.origin_secret.result
    }

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "https-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  default_cache_behavior {
    target_origin_id       = "APIGatewayOrigin"
    viewer_protocol_policy = "redirect-to-https"
    allowed_methods        = ["GET", "HEAD", "OPTIONS", "PUT", "POST", "PATCH", "DELETE"]
    cached_methods         = ["GET", "HEAD"]

    # Managed-CachingDisabled: API responses must not be cached
    cache_policy_id = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad"
    # Managed-AllViewerExceptHostHeader: forward auth/cors headers, exclude Host
    origin_request_policy_id = "b689b0a8-53d0-40ab-baf2-68738e2966ac"
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    acm_certificate_arn      = aws_acm_certificate_validation.api_cf.certificate_arn
    ssl_support_method       = "sni-only"
    minimum_protocol_version = "TLSv1.2_2021"
  }

  tags = local.common_tags
}
