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
		// All resources created with this provider will automatically receive these default tags:
		// - Project: The Pulumi project name
		// - ManagedBy: "Pulumi"
		// - Environment: The Pulumi stack name
		// - Contact: "Joel@joelgonzaga.com"
		// - CreatedBy: "Udacity Meme Generator - Pulumi Network"
		awsProvider, err := aws.NewProvider(ctx, "custom-provider", &aws.ProviderArgs{
			Region: pulumi.String(os.Getenv("AWS_REGION")),
			DefaultTags: &aws.ProviderDefaultTagsArgs{
				Tags: pulumi.StringMap{
					"Project":     pulumi.String(ctx.Project()),
					"ManagedBy":   pulumi.String("Pulumi"),
					"Environment": pulumi.String(ctx.Stack()),
					"Contact":     pulumi.String("Joel@joelgonzaga.com"),
					"CreatedBy":   pulumi.String("Udacity Meme Generator - Pulumi Network"),
				},
			},
		})
		if err != nil {
			return err
		}

		vpcId := os.Getenv("TARGET_VPC_NAME")
		if vpcId == "" {
			return fmt.Errorf("TARGET_VPC_NAME environment variable is required")
		}

		igw, err := ec2.NewInternetGateway(ctx, "network-igw", &ec2.InternetGatewayArgs{
			VpcId: pulumi.String(vpcId),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("Network-InternetGateway"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		publicSubnet1, err := ec2.NewSubnet(ctx, "public-subnet-1", &ec2.SubnetArgs{
			VpcId:            pulumi.String(vpcId),
			CidrBlock:        pulumi.String("10.8.1.0/24"),
			AvailabilityZone: pulumi.String("us-west-2a"),
			MapPublicIpOnLaunch: pulumi.Bool(true),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("Public-Subnet-1"),
				"Type": pulumi.String("Public"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		publicSubnet2, err := ec2.NewSubnet(ctx, "public-subnet-2", &ec2.SubnetArgs{
			VpcId:            pulumi.String(vpcId),
			CidrBlock:        pulumi.String("10.8.2.0/24"),
			AvailabilityZone: pulumi.String("us-west-2b"),
			MapPublicIpOnLaunch: pulumi.Bool(true),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("Public-Subnet-2"),
				"Type": pulumi.String("Public"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		privateSubnet1, err := ec2.NewSubnet(ctx, "private-subnet-1", &ec2.SubnetArgs{
			VpcId:            pulumi.String(vpcId),
			CidrBlock:        pulumi.String("10.8.10.0/24"),
			AvailabilityZone: pulumi.String("us-west-2a"),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("Private-Subnet-1"),
				"Type": pulumi.String("Private"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		privateSubnet2, err := ec2.NewSubnet(ctx, "private-subnet-2", &ec2.SubnetArgs{
			VpcId:            pulumi.String(vpcId),
			CidrBlock:        pulumi.String("10.8.11.0/24"),
			AvailabilityZone: pulumi.String("us-west-2b"),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("Private-Subnet-2"),
				"Type": pulumi.String("Private"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// 8. Create Route Table
		routeTable, err := ec2.NewRouteTable(ctx, "network-route-table", &ec2.RouteTableArgs{
			VpcId: pulumi.String(vpcId),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("Network-RouteTable"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}


		_, err = ec2.NewRoute(ctx, "igw-route", &ec2.RouteArgs{
			RouteTableId:         routeTable.ID(),
			DestinationCidrBlock: pulumi.String("0.0.0.0/0"),
			GatewayId:            igw.ID(),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}


		_, err = ec2.NewRouteTableAssociation(ctx, "public-subnet-1-association", &ec2.RouteTableAssociationArgs{
			SubnetId:     publicSubnet1.ID(),
			RouteTableId: routeTable.ID(),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		_, err = ec2.NewRouteTableAssociation(ctx, "public-subnet-2-association", &ec2.RouteTableAssociationArgs{
			SubnetId:     publicSubnet2.ID(),
			RouteTableId: routeTable.ID(),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		_, err = ec2.NewRouteTableAssociation(ctx, "private-subnet-1-association", &ec2.RouteTableAssociationArgs{
			SubnetId:     privateSubnet1.ID(),
			RouteTableId: routeTable.ID(),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		_, err = ec2.NewRouteTableAssociation(ctx, "private-subnet-2-association", &ec2.RouteTableAssociationArgs{
			SubnetId:     privateSubnet2.ID(),
			RouteTableId: routeTable.ID(),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// 11. Export outputs
		ctx.Export("vpcId", pulumi.String(vpcId))
		ctx.Export("internetGatewayId", igw.ID())
		ctx.Export("routeTableId", routeTable.ID())
		ctx.Export("publicSubnet1Id", publicSubnet1.ID())
		ctx.Export("publicSubnet2Id", publicSubnet2.ID())
		ctx.Export("privateSubnet1Id", privateSubnet1.ID())
		ctx.Export("privateSubnet2Id", privateSubnet2.ID())

		return nil
	})
}

