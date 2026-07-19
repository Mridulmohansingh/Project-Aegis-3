terraform {
  required_version = ">= 1.5.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
  backend "s3" {
    bucket         = "aegis-terraform-state-prod"
    key            = "platform/terraform.tfstate"
    region         = "us-east-1"
    dynamodb_table = "aegis-terraform-locks"
    encrypt        = true
  }
}

provider "aws" {
  region = var.aws_region
  default_tags {
    tags = {
      Project     = "AEGIS"
      Environment = var.environment
      ManagedBy   = "Terraform"
    }
  }
}

module "vpc" {
  source = "./modules/vpc"

  vpc_cidr             = var.vpc_cidr
  environment          = var.environment
  public_subnets_cidr  = var.public_subnets_cidr
  private_subnets_cidr = var.private_subnets_cidr
  availability_zones   = var.availability_zones
}

module "eks" {
  source = "./modules/eks"

  cluster_name       = "aegis-${var.environment}-cluster"
  vpc_id             = module.vpc.vpc_id
  private_subnet_ids = module.vpc.private_subnet_ids
  environment        = var.environment
}

module "rds" {
  source = "./modules/rds"

  db_name               = "aegisdb"
  environment           = var.environment
  vpc_id                = module.vpc.vpc_id
  private_subnet_ids    = module.vpc.private_subnet_ids
  eks_security_group_id = module.eks.cluster_security_group_id
}

resource "aws_s3_bucket" "exam_artifacts" {
  bucket = "aegis-exam-artifacts-${var.environment}"
}

resource "aws_s3_bucket_server_side_encryption_configuration" "exam_artifacts_sse" {
  bucket = aws_s3_bucket.exam_artifacts.id
  rule {
    apply_server_side_encryption_by_default {
      sse_algorithm = "AES256"
    }
  }
}

resource "aws_s3_bucket_public_access_block" "exam_artifacts_pab" {
  bucket                  = aws_s3_bucket.exam_artifacts.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}
