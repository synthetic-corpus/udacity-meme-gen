package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi-docker/sdk/v4/go/docker"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// 1. Define the Explicit Provider with Default Tags
		awsProvider, err := aws.NewProvider(ctx, "custom-provider", &aws.ProviderArgs{
			Region: pulumi.String(os.Getenv("AWS_REGION")),
			DefaultTags: &aws.ProviderDefaultTagsArgs{
				Tags: pulumi.StringMap{
					"Project":     pulumi.String(ctx.Project()),
					"ManagedBy":   pulumi.String("Pulumi"),
					"Environment": pulumi.String(ctx.Stack()),
					"Contact": pulumi.String("Joel@joelgonzaga.com"),
					"CreatedBy": pulumi.String("Udacity Meme Generator - Pulumi Deployment"),
				},
			},
		})
		if err != nil {
			return err
		}
		// Create a new VPC subnet
		// Get the VPC ID from the environment variable
		vpcId := os.Getenv("TARGET_VPC_NAME")
		if vpcId == "" {
			return fmt.Errorf("TARGET_VPC_NAME environment variable is required")
		}

		// Create a new subnet in that VPC
		subnet, err := ec2.NewSubnet(ctx, "pulumi-managed-subnet", &ec2.SubnetArgs{
			VpcId:     pulumi.String(vpcId),
			CidrBlock: pulumi.String("10.8.101.0/24"), // Ensure this doesn't overlap
			Tags: pulumi.StringMap{
				"Name": pulumi.String("Pulumi-Subnet"),
			},
		}, pulumi.Provider(awsProvider)) // awsProvider is the explicit provider with default tags
		if err != nil {
			return err
		}

		// 1. Retrieve the ECR Repository URL from the environment variable
		ecrRepoUrl := os.Getenv("ECR_WEB_REPO")
		if ecrRepoUrl == "" {
			return fmt.Errorf("ECR_WEB_REPO environment variable is not set")
		}

		// 2. Create Docker provider for building and pushing images
		// The Docker provider needs access to the Docker daemon (mounted via docker socket)
		dockerProvider, err := docker.NewProvider(ctx, "docker-provider", &docker.ProviderArgs{
			// When running in Docker, the provider will use the mounted Docker socket
		})
		if err != nil {
			return err
		}

		// 3. Extract the server URL from the ECR repository URL (domain only, without the repo path)
		// ECR URL format: account.dkr.ecr.region.amazonaws.com/repo-name
		// Server should be just the domain part
		ecrServer := ecrRepoUrl
		if idx := strings.LastIndex(ecrRepoUrl, "/"); idx != -1 {
			ecrServer = ecrRepoUrl[:idx]
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

		// Export the new Subnet ID and the ECR Repository URL
		ctx.Export("subnetId", subnet.ID())
		ctx.Export("fullImageName", image.ImageName)
		return nil
	})
}
