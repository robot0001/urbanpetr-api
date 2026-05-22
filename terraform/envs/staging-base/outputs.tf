output "db_app_secret_arn" {
  value       = aws_secretsmanager_secret.db_app.arn
  description = "Staging app DB secret ARN."
}

output "db_migrator_secret_arn" {
  value       = aws_secretsmanager_secret.db_migrator.arn
  description = "Staging migrator DB secret ARN."
}

output "db_readonly_secret_arn" {
  value       = aws_secretsmanager_secret.db_readonly.arn
  description = "Staging readonly DB secret ARN."
}

output "nat_eip" {
  value       = aws_eip.nat.public_ip
  description = "Staging NAT Gateway Elastic IP."
}
