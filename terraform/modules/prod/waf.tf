resource "random_password" "origin_secret" {
  length  = 32
  special = false
}

# Empty by default — populated with 0.0.0.0/0 and ::/0 by the kill-switch Lambda
# to block all traffic at the CloudFront edge within seconds.
resource "aws_wafv2_ip_set" "kill_switch" {
  provider           = aws.us_east_1
  name               = "urbanpetr-kill-switch"
  scope              = "CLOUDFRONT"
  ip_address_version = "IPV4"
  addresses          = []

  tags = local.common_tags
}

resource "aws_wafv2_ip_set" "kill_switch_v6" {
  provider           = aws.us_east_1
  name               = "urbanpetr-kill-switch-v6"
  scope              = "CLOUDFRONT"
  ip_address_version = "IPV6"
  addresses          = []

  tags = local.common_tags
}

resource "aws_wafv2_web_acl" "shared" {
  provider = aws.us_east_1
  name     = "urbanpetr-shared"
  scope    = "CLOUDFRONT"

  default_action {
    allow {}
  }

  # Priority 0 — evaluated first; no-op while IP sets are empty.
  # Populated by kill-switch Lambda to block all traffic instantly.
  rule {
    name     = "KillSwitch"
    priority = 0
    action {
      block {}
    }
    statement {
      or_statement {
        statement {
          ip_set_reference_statement {
            arn = aws_wafv2_ip_set.kill_switch.arn
          }
        }
        statement {
          ip_set_reference_statement {
            arn = aws_wafv2_ip_set.kill_switch_v6.arn
          }
        }
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "KillSwitch"
      sampled_requests_enabled   = true
    }
  }

  rule {
    name     = "AWSManagedRulesAmazonIpReputationList"
    priority = 1
    override_action {
      none {}
    }
    statement {
      managed_rule_group_statement {
        name        = "AWSManagedRulesAmazonIpReputationList"
        vendor_name = "AWS"
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "IpReputationList"
      sampled_requests_enabled   = true
    }
  }

  rule {
    name     = "AWSManagedRulesCommonRuleSet"
    priority = 2
    override_action {
      none {}
    }
    statement {
      managed_rule_group_statement {
        name        = "AWSManagedRulesCommonRuleSet"
        vendor_name = "AWS"

        # SizeRestrictions_Body blocks any body > 8 KB (WAF's default inspection
        # limit). File uploads are always larger — API Gateway enforces its own
        # 10 MB limit, so blocking here just prevents uploads without adding
        # meaningful security.
        rule_action_override {
          name = "SizeRestrictions_Body"
          action_to_use {
            count {}
          }
        }
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "CommonRuleSet"
      sampled_requests_enabled   = true
    }
  }

  rule {
    name     = "IPRateLimit"
    priority = 3
    action {
      block {}
    }
    statement {
      rate_based_statement {
        limit              = 1000
        aggregate_key_type = "IP"
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "IPRateLimit"
      sampled_requests_enabled   = true
    }
  }

  visibility_config {
    cloudwatch_metrics_enabled = true
    metric_name                = "urbanpetr-shared-waf"
    sampled_requests_enabled   = true
  }

  tags = local.common_tags
}
