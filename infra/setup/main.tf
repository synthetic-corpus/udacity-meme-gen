terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.23.0"
    }
  }

  backend "s3" {
    bucket         = "jtg-terraform-buckets"
    key            = "meme-generator-app-one-setup"
    region         = "us-west-2"
    encrypt        = true
    dynamodb_table = "terraform-lock-table"
  }
}

provider "aws" {
  region = "us-west-2"
  default_tags {
    tags = {
      Environment = terraform.workspace
      Project     = "Udacity Meme Generator - Pulumi Deployment"
      contact     = "Joel@joelgonzaga.com"
      ManagedBy   = "Terraform/setup"
    }
  }
}

output "ecr_repository_url" {
  description = "The URL of the ECR repository"
  value       = aws_ecr_repository.meme_generator_app.repository_url
}

output "ecr_repository_id" {
  description = "The ID (ARN) of the ECR repository"
  value       = aws_ecr_repository.meme_generator_app.arn
}