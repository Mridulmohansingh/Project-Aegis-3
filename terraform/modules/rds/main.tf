resource "aws_db_subnet_group" "rds" {
  name       = "aegis-${var.environment}-rds-subnet-group"
  subnet_ids = var.private_subnet_ids

  tags = {
    Name = "AEGIS RDS subnet group"
  }
}

resource "aws_security_group" "rds" {
  name        = "aegis-${var.environment}-rds-sg"
  description = "Allow EKS traffic to RDS"
  vpc_id      = var.vpc_id

  ingress {
    description     = "PostgreSQL from EKS"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [var.eks_security_group_id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_kms_key" "rds_key" {
  description             = "KMS key for AEGIS RDS"
  deletion_window_in_days = 10
  enable_key_rotation     = true
}

resource "aws_db_parameter_group" "rds" {
  name   = "aegis-${var.environment}-pg16"
  family = "postgres16"

  parameter {
    name  = "shared_preload_libraries"
    value = "pg_stat_statements"
  }
}

resource "aws_db_instance" "rds" {
  identifier                  = "aegis-${var.environment}-db"
  engine                      = "postgres"
  engine_version              = "16"
  instance_class              = "db.r6g.xlarge"
  allocated_storage           = 100
  max_allocated_storage       = 1000
  storage_type                = "io1"
  iops                        = 3000
  storage_encrypted           = true
  kms_key_id                  = aws_kms_key.rds_key.arn
  db_name                     = var.db_name
  username                    = "postgresadmin"
  manage_master_user_password = true
  
  db_subnet_group_name   = aws_db_subnet_group.rds.name
  vpc_security_group_ids = [aws_security_group.rds.id]
  parameter_group_name   = aws_db_parameter_group.rds.name

  multi_az = true
  
  backup_retention_period = 35
  backup_window           = "03:00-04:00"
  maintenance_window      = "Sun:04:00-Sun:05:00"
  
  performance_insights_enabled    = true
  performance_insights_kms_key_id = aws_kms_key.rds_key.arn

  skip_final_snapshot = false
  final_snapshot_identifier = "aegis-${var.environment}-db-final-snapshot"
}

variable "db_name" { type = string }
variable "environment" { type = string }
variable "vpc_id" { type = string }
variable "private_subnet_ids" { type = list(string) }
variable "eks_security_group_id" { type = string }
