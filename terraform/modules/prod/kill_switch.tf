data "aws_caller_identity" "current" {}

# Everything the kill switch takes down. football-api shares the WAF and the
# account, so a spike on either API kills both projects.
locals {
  kill_switch_lambda_names = [
    "urbanpetr-api-prod",
    "football-api-prod",
  ]

  kill_switch_cf_dist_ids = [
    aws_cloudfront_distribution.api.id,
    data.terraform_remote_state.website.outputs.cloudfront_distribution_id,
    data.terraform_remote_state.admin.outputs.cloudfront_distribution_id,
    data.terraform_remote_state.football_api.outputs.api_cloudfront_distribution_id,
    data.terraform_remote_state.football_api.outputs.images_cloudfront_distribution_id,
    data.terraform_remote_state.football_web.outputs.cloudfront_distribution_id,
    data.terraform_remote_state.football_admin.outputs.cloudfront_distribution_id,
  ]
}

data "archive_file" "kill_switch" {
  type        = "zip"
  source_file = "${path.module}/../../../lambda/kill_switch/main.py"
  output_path = "${path.module}/../../../lambda/kill_switch/main.zip"
}

resource "aws_cloudwatch_log_group" "kill_switch_lambda" {
  name              = "/aws/lambda/urbanpetr-kill-switch"
  retention_in_days = 7
  tags              = local.common_tags
}

resource "aws_iam_role" "kill_switch_lambda" {
  name               = "urbanpetr-kill-switch"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json
  tags               = local.common_tags
}

data "aws_iam_policy_document" "kill_switch_lambda" {
  statement {
    sid     = "ThrottleApiLambdas"
    effect  = "Allow"
    actions = ["lambda:PutFunctionConcurrency"]
    resources = [
      for name in local.kill_switch_lambda_names :
      "arn:aws:lambda:eu-central-1:${data.aws_caller_identity.current.account_id}:function:${name}"
    ]
  }

  statement {
    sid     = "BlockWAF"
    effect  = "Allow"
    actions = ["wafv2:GetIPSet", "wafv2:UpdateIPSet"]
    resources = [
      aws_wafv2_ip_set.kill_switch.arn,
      aws_wafv2_ip_set.kill_switch_v6.arn,
    ]
  }

  statement {
    sid     = "DisableCloudFront"
    effect  = "Allow"
    actions = ["cloudfront:GetDistributionConfig", "cloudfront:UpdateDistribution"]
    resources = [
      for id in local.kill_switch_cf_dist_ids :
      "arn:aws:cloudfront::${data.aws_caller_identity.current.account_id}:distribution/${id}"
    ]
  }

  statement {
    sid       = "WriteLogs"
    effect    = "Allow"
    actions   = ["logs:CreateLogStream", "logs:PutLogEvents"]
    resources = ["${aws_cloudwatch_log_group.kill_switch_lambda.arn}:*"]
  }
}

resource "aws_iam_role_policy" "kill_switch_lambda" {
  name   = "kill-switch-actions"
  role   = aws_iam_role.kill_switch_lambda.id
  policy = data.aws_iam_policy_document.kill_switch_lambda.json
}

resource "aws_lambda_function" "kill_switch" {
  function_name    = "urbanpetr-kill-switch"
  role             = aws_iam_role.kill_switch_lambda.arn
  runtime          = "python3.12"
  handler          = "main.handler"
  filename         = data.archive_file.kill_switch.output_path
  source_code_hash = data.archive_file.kill_switch.output_base64sha256
  timeout          = 90

  environment {
    variables = {
      LAMBDA_NAMES       = join(",", local.kill_switch_lambda_names)
      WAF_IP_SET_ID      = aws_wafv2_ip_set.kill_switch.id
      WAF_IP_SET_NAME    = aws_wafv2_ip_set.kill_switch.name
      WAF_IP_SET_V6_ID   = aws_wafv2_ip_set.kill_switch_v6.id
      WAF_IP_SET_V6_NAME = aws_wafv2_ip_set.kill_switch_v6.name
      CF_DIST_IDS        = join(",", local.kill_switch_cf_dist_ids)
    }
  }

  tags = local.common_tags
}

resource "aws_lambda_permission" "kill_switch_sns" {
  statement_id  = "AllowSNSInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.kill_switch.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.kill_switch.arn
}

resource "aws_sns_topic_subscription" "kill_switch" {
  topic_arn = aws_sns_topic.kill_switch.arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.kill_switch.arn
}
