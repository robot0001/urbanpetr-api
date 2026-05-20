# SSM Bastion — tiny ARM64 EC2 instance in a private subnet used exclusively
# as a jump point for port-forwarding to RDS via SSM Session Manager.
# No inbound rules; access is controlled by IAM (ssm:StartSession).

data "aws_ami" "al2023_arm" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["al2023-ami-2023.*-arm64"]
  }
}

resource "aws_security_group" "ssm_bastion" {
  name        = "${var.project_name}-${var.environment}-ssm-bastion"
  description = "SSM bastion - no inbound, egress to RDS on 5432 and HTTPS for SSM"
  vpc_id      = local.vpc_id

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${var.environment}-ssm-bastion"
  })
}

resource "aws_vpc_security_group_egress_rule" "bastion_to_rds" {
  security_group_id            = aws_security_group.ssm_bastion.id
  referenced_security_group_id = local.rds_security_group_id
  from_port                    = 5432
  to_port                      = 5432
  ip_protocol                  = "tcp"
  description                  = "SSM bastion to RDS"

  tags = local.common_tags
}

resource "aws_vpc_security_group_egress_rule" "bastion_to_ssm" {
  security_group_id = aws_security_group.ssm_bastion.id
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 443
  to_port           = 443
  ip_protocol       = "tcp"
  description       = "SSM agent outbound to SSM endpoints via NAT"

  tags = local.common_tags
}

resource "aws_iam_role" "ssm_bastion" {
  name = "${var.project_name}-${var.environment}-ssm-bastion"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })

  tags = local.common_tags
}

resource "aws_iam_role_policy_attachment" "ssm_bastion" {
  role       = aws_iam_role.ssm_bastion.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

resource "aws_iam_instance_profile" "ssm_bastion" {
  name = "${var.project_name}-${var.environment}-ssm-bastion"
  role = aws_iam_role.ssm_bastion.name

  tags = local.common_tags
}

resource "aws_instance" "ssm_bastion" {
  ami                    = data.aws_ami.al2023_arm.id
  instance_type          = "t4g.nano"
  subnet_id              = local.private_subnet_ids[0]
  vpc_security_group_ids = [aws_security_group.ssm_bastion.id]
  iam_instance_profile   = aws_iam_instance_profile.ssm_bastion.name

  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
  }

  root_block_device {
    volume_size = 8
    volume_type = "gp3"
    encrypted   = true
  }

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${var.environment}-ssm-bastion"
  })
}

resource "aws_vpc_security_group_ingress_rule" "bastion_to_rds" {
  security_group_id            = local.rds_security_group_id
  referenced_security_group_id = aws_security_group.ssm_bastion.id
  from_port                    = 5432
  to_port                      = 5432
  ip_protocol                  = "tcp"
  description                  = "SSM bastion to RDS"

  tags = local.common_tags
}
