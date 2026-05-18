# EC2 Instance Connect Endpoint — lets you tunnel to RDS from your Mac
# without a bastion host or public IP. Access is controlled by IAM
# (ec2-instance-connect:OpenTunnel).

resource "aws_security_group" "eic_endpoint" {
  name        = "${var.project_name}-${var.environment}-eic-endpoint"
  description = "EC2 Instance Connect Endpoint — egress to RDS only"
  vpc_id      = local.vpc_id

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${var.environment}-eic-endpoint"
  })
}

resource "aws_vpc_security_group_egress_rule" "eic_to_rds" {
  security_group_id            = aws_security_group.eic_endpoint.id
  referenced_security_group_id = local.rds_security_group_id
  from_port                    = 5432
  to_port                      = 5432
  ip_protocol                  = "tcp"
  description                  = "EIC endpoint to RDS"

  tags = local.common_tags
}

resource "aws_ec2_instance_connect_endpoint" "main" {
  subnet_id          = local.private_subnet_ids[0]
  security_group_ids = [aws_security_group.eic_endpoint.id]
  preserve_client_ip = false

  tags = merge(local.common_tags, {
    Name = "${var.project_name}-${var.environment}-eic-endpoint"
  })
}

resource "aws_vpc_security_group_ingress_rule" "eic_to_rds" {
  security_group_id            = local.rds_security_group_id
  referenced_security_group_id = aws_security_group.eic_endpoint.id
  from_port                    = 5432
  to_port                      = 5432
  ip_protocol                  = "tcp"
  description                  = "EIC endpoint to RDS"

  tags = local.common_tags
}
