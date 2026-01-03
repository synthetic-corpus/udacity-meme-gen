package main

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
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

		// Export the new Subnet ID
		ctx.Export("subnetId", subnet.ID())
		return nil
	})
}
