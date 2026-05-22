data "aws_caller_identity" "current" {}

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
    sid       = "ThrottleApiLambda"
    effect    = "Allow"
    actions   = ["lambda:PutFunctionConcurrency"]
    resources = ["arn:aws:lambda:eu-central-1:${data.aws_caller_identity.current.account_id}:function:urbanpetr-api-prod"]
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
      "arn:aws:cloudfront::${data.aws_caller_identity.current.account_id}:distribution/${aws_cloudfront_distribution.api.id}",
      "arn:aws:cloudfront::${data.aws_caller_identity.current.account_id}:distribution/${data.terraform_remote_state.website.outputs.cloudfront_distribution_id}",
      "arn:aws:cloudfront::${data.aws_caller_identity.current.account_id}:distribution/${data.terraform_remote_state.admin.outputs.cloudfront_distribution_id}",
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
  timeout          = 30

  environment {
    variables = {
      API_LAMBDA_NAME    = "urbanpetr-api-prod"
      WAF_IP_SET_ID      = aws_wafv2_ip_set.kill_switch.id
      WAF_IP_SET_NAME    = aws_wafv2_ip_set.kill_switch.name
      WAF_IP_SET_V6_ID   = aws_wafv2_ip_set.kill_switch_v6.id
      WAF_IP_SET_V6_NAME = aws_wafv2_ip_set.kill_switch_v6.name
      API_CF_DIST_ID     = aws_cloudfront_distribution.api.id
      WEBSITE_CF_DIST_ID = data.terraform_remote_state.website.outputs.cloudfront_distribution_id
      ADMIN_CF_DIST_ID   = data.terraform_remote_state.admin.outputs.cloudfront_distribution_id
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
