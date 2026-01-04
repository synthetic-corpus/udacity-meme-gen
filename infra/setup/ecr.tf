############################################
# ECR Repositories for Meme Generator App  #
############################################

resource "aws_ecr_repository" "meme_generator_app" {
  name                 = "meme-generator-app"
  image_tag_mutability = "MUTABLE"
  force_delete         = true

  image_scanning_configuration {
    scan_on_push = false
  }
}