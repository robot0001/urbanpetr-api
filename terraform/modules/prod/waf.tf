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

        # Exclude file-upload endpoints from all CommonRuleSet body-inspection
        # rules. Binary content (a ZIP, an image) triggers false positives
        # (SizeRestrictions, XSS, SQLi, RFI rules) because WAF inspects raw
        # bytes, not the file format. Each excluded endpoint is protected by
        # its own auth (JWT / football-api session) and a request size cap
        # enforced by the app itself. All other paths retain full
        # CommonRuleSet protection.
        scope_down_statement {
          not_statement {
            statement {
              or_statement {
                statement {
                  byte_match_statement {
                    search_string         = "/v1/history/youtube/ingest"
                    positional_constraint = "EXACTLY"
                    field_to_match {
                      uri_path {}
                    }
                    text_transformation {
                      priority = 0
                      type     = "NONE"
                    }
                  }
                }
                statement {
                  byte_match_statement {
                    search_string         = "/v1/image"
                    positional_constraint = "EXACTLY"
                    field_to_match {
                      uri_path {}
                    }
                    text_transformation {
                      priority = 0
                      type     = "NONE"
                    }
                  }
                }
              }
            }
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

  rule {
    name     = "AWSManagedRulesKnownBadInputsRuleSet"
    priority = 4
    override_action {
      none {}
    }
    statement {
      managed_rule_group_statement {
        name        = "AWSManagedRulesKnownBadInputsRuleSet"
        vendor_name = "AWS"

        # Same exclusion, same reason as CommonRuleSet above: several rules in
        # this group (Log4JRCE, the JavaDeserializationRCE family) inspect the
        # request body, and raw ZIP/image bytes trip them. Keeping the two
        # groups' scope-downs identical means an upload endpoint is either
        # excluded from body inspection everywhere or nowhere — a path excluded
        # from one but not the other would fail in a way nobody would predict
        # from reading either rule alone.
        scope_down_statement {
          not_statement {
            statement {
              or_statement {
                statement {
                  byte_match_statement {
                    search_string         = "/v1/history/youtube/ingest"
                    positional_constraint = "EXACTLY"
                    field_to_match {
                      uri_path {}
                    }
                    text_transformation {
                      priority = 0
                      type     = "NONE"
                    }
                  }
                }
                statement {
                  byte_match_statement {
                    search_string         = "/v1/image"
                    positional_constraint = "EXACTLY"
                    field_to_match {
                      uri_path {}
                    }
                    text_transformation {
                      priority = 0
                      type     = "NONE"
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "KnownBadInputs"
      sampled_requests_enabled   = true
    }
  }

  # A second, much tighter rate limit for the two credential-facing routes on
  # football-api, which sits behind this same shared ACL. IPRateLimit above is
  # sized for ordinary browsing (1000 req / 5 min covers a page load's worth of
  # assets several times over); at that budget an attacker still gets ~200
  # credential attempts a minute, which is a working brute-force rate.
  #
  # 100 requests / 5 min / IP. Both routes are called once per interactive
  # sign-in and the session token is then reused, so a real user spends 1-2 of
  # these per session; 100 leaves room for a whole office behind one NAT egress
  # IP and still cuts an attacker's throughput 10x. AWS accepts a limit as low
  # as 10, so this is nowhere near the floor — it can be tightened later from
  # the CloudWatch "AuthRateLimit" metric rather than guessed tighter now.
  #
  # Evaluation window is stated explicitly because the number is meaningless
  # without it: WAF counts over a trailing window, 300s being the default.
  #
  # Scoped by URI path only, deliberately. This ACL is shared with
  # urbanpetr.com and the football front end, neither of which serves these
  # paths, so a path scope cannot catch their traffic; adding a Host match
  # would couple this file to football-api's domain names and would silently
  # stop matching the day one of them changes. Everything outside the
  # scope-down keeps the 1000/5min budget untouched.
  rule {
    name     = "AuthRateLimit"
    priority = 5
    action {
      block {}
    }
    statement {
      rate_based_statement {
        limit                 = 100
        aggregate_key_type    = "IP"
        evaluation_window_sec = 300

        scope_down_statement {
          or_statement {
            # POST /v1/session — exchanges a Cognito ID token for a session
            # token. DELETE /v1/session shares the path and is counted too;
            # that is harmless, sign-out is just as low-volume as sign-in.
            statement {
              byte_match_statement {
                search_string         = "/v1/session"
                positional_constraint = "EXACTLY"
                field_to_match {
                  uri_path {}
                }
                # URL_DECODE then LOWERCASE: WAF matches the raw URI, but Go
                # routes on the percent-decoded path, so "/v1/%73ession" would
                # reach the handler while an untransformed byte match missed
                # it. The existing CommonRuleSet scope-down can use NONE
                # because a miss there means *more* inspection; a miss here
                # means no rate limit at all, so it has to normalise first.
                text_transformation {
                  priority = 0
                  type     = "URL_DECODE"
                }
                text_transformation {
                  priority = 1
                  type     = "LOWERCASE"
                }
              }
            }
            # POST /v1/account/invite/{token}/redeem — the only route that
            # grants access to someone who has none. The token is a path
            # segment, so there is no exact string to match on: match the
            # fixed prefix and the fixed suffix together instead. Prefix alone
            # would also sweep in DELETE /v1/account/invite/{uuid}, and suffix
            # alone would match any future "/redeem" route on any site behind
            # this shared ACL.
            statement {
              and_statement {
                statement {
                  byte_match_statement {
                    search_string         = "/v1/account/invite/"
                    positional_constraint = "STARTS_WITH"
                    field_to_match {
                      uri_path {}
                    }
                    text_transformation {
                      priority = 0
                      type     = "URL_DECODE"
                    }
                    text_transformation {
                      priority = 1
                      type     = "LOWERCASE"
                    }
                  }
                }
                statement {
                  byte_match_statement {
                    search_string         = "/redeem"
                    positional_constraint = "ENDS_WITH"
                    field_to_match {
                      uri_path {}
                    }
                    text_transformation {
                      priority = 0
                      type     = "URL_DECODE"
                    }
                    text_transformation {
                      priority = 1
                      type     = "LOWERCASE"
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
    visibility_config {
      cloudwatch_metrics_enabled = true
      metric_name                = "AuthRateLimit"
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
