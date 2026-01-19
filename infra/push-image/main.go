package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pulumi/pulumi-docker/sdk/v4/go/docker" //nolint
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"      //nolint
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// 1. Retrieve the ECR Repository URL from the environment variable
		ecrRepoUrl := os.Getenv("ECR_WEB_REPO")
		if ecrRepoUrl == "" {
			return fmt.Errorf("ECR_WEB_REPO environment variable is not set")
		}

		// 2. Authenticate Docker with ECR before building/pushing
		// Extract the server URL from the ECR repository URL (domain only, without the repo path)
		// ECR URL format: account.dkr.ecr.region.amazonaws.com/repo-name
		ecrServer := ecrRepoUrl
		if idx := strings.LastIndex(ecrRepoUrl, "/"); idx != -1 {
			ecrServer = ecrRepoUrl[:idx]
		}

		// Get AWS region
		awsRegion := os.Getenv("AWS_REGION")
		if awsRegion == "" {
			awsRegion = "us-west-2"
		}

		// Authenticate Docker with ECR using AWS CLI
		// This runs: aws ecr get-login-password --region <region> | docker login --username AWS --password-stdin <ecr-server>
		ctx.Log.Info(fmt.Sprintf("Authenticating Docker with ECR: %s", ecrServer), nil)
		
		// Get ECR password
		getPasswordCmd := exec.Command("aws", "ecr", "get-login-password", "--region", awsRegion)
		getPasswordCmd.Env = os.Environ()
		passwordOutput, err := getPasswordCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to get ECR login password: %w. Make sure AWS CLI is installed and AWS credentials are configured", err)
		}
		password := strings.TrimSpace(string(passwordOutput))

		// Login to Docker registry
		dockerLoginCmd := exec.Command("docker", "login", "--username", "AWS", "--password-stdin", ecrServer)
		dockerLoginCmd.Env = os.Environ()
		dockerLoginCmd.Stdin = strings.NewReader(password)
		dockerLoginCmd.Stdout = os.Stdout
		dockerLoginCmd.Stderr = os.Stderr
		if err := dockerLoginCmd.Run(); err != nil {
			return fmt.Errorf("failed to login to ECR: %w", err)
		}

		ctx.Log.Info("Successfully authenticated with ECR", nil)

		// 3. Create Docker provider for building and pushing images
		// The Docker provider needs access to the Docker daemon (mounted via docker socket)
		dockerProvider, err := docker.NewProvider(ctx, "docker-provider", &docker.ProviderArgs{
			// When running in Docker, the provider will use the mounted Docker socket
		})
		if err != nil {
			return err
		}

		// 4. Build and Push the Docker image from src-test folder
		// The src-test folder is mounted at /proj/src-test in the container
		// Since working_dir is /proj, we reference it as "src-test"
		image, err := docker.NewImage(ctx, "meme-generator-app", &docker.ImageArgs{
			Build: &docker.DockerBuildArgs{
				Context: pulumi.String("src-test"), // Path relative to working directory /proj
			},
			ImageName: pulumi.String(ecrRepoUrl + ":latest"),
			Registry: &docker.RegistryArgs{
				Server: pulumi.String(ecrServer),
			},
		}, pulumi.Provider(dockerProvider))
		if err != nil {
			return err
		}

		// Export the image name
		ctx.Export("imageName", image.ImageName)
		ctx.Export("ecrRepoUrl", pulumi.String(ecrRepoUrl))

		return nil
	})
}

