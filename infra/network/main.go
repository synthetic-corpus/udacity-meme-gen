package main

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"      //nolint
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"  //nolint
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"      //nolint
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
				// Human friendly tags
				"Name": pulumi.String("Public-Subnet-1"),
				"Type": pulumi.String("Public"),
				// required by ELB, k8s, karpenter etc
				"kubernetes.io/role/elb": pulumi.String("1"),
				"kubernetes.io/cluster/meme-generator-cluster": pulumi.String("shared"),
				"karpenter.sh/discovery": pulumi.String("meme-generator-cluster"),
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
				// Human Friendly tags
				"Name": pulumi.String("Public-Subnet-2"),
				"Type": pulumi.String("Public"),
				// required by ELB, k8s, karpenter etc
				"kubernetes.io/role/elb": pulumi.String("1"),
				"kubernetes.io/cluster/meme-generator-cluster": pulumi.String("shared"),
				"karpenter.sh/discovery": pulumi.String("meme-generator-cluster"),
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
				// Generic Human friendly tags
				"Name": pulumi.String("Private-Subnet-1"),
				"Type": pulumi.String("Private"),
				// required by ELB, k8s, karpenter etc
				"kubernetes.io/role/internal-elb": pulumi.String("1"),
				"kubernetes.io/cluster/meme-generator-cluster": pulumi.String("shared"),
				"karpenter.sh/discovery": pulumi.String("meme-generator-cluster"),
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
				// Generic and human readable tags
				"Name": pulumi.String("Private-Subnet-2"),
				"Type": pulumi.String("Private"),
				// required by ELB, k8s, karpenter etc
				"kubernetes.io/role/internal-elb": pulumi.String("1"),
				"kubernetes.io/cluster/meme-generator-cluster": pulumi.String("shared"),
				"karpenter.sh/discovery": pulumi.String("meme-generator-cluster"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// 8. Create Route Table for Public Subnets (with Internet Gateway route)
		publicRouteTable, err := ec2.NewRouteTable(ctx, "public-route-table", &ec2.RouteTableArgs{
			VpcId: pulumi.String(vpcId),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("Public-Subnets-RouteTable"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// Add route to Internet Gateway for public subnets
		_, err = ec2.NewRoute(ctx, "public-igw-route", &ec2.RouteArgs{
			RouteTableId:         publicRouteTable.ID(),
			DestinationCidrBlock: pulumi.String("0.0.0.0/0"),
			GatewayId:            igw.ID(),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// 9. Create Route Table for Private Subnets
		// Private subnets use NAT Gateway for outbound internet (no direct IGW route)
		privateRouteTable, err := ec2.NewRouteTable(ctx, "private-route-table", &ec2.RouteTableArgs{
			VpcId: pulumi.String(vpcId),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("Private-Subnets-RouteTable"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// 9a. Create Elastic IP for NAT Gateway
		natEip, err := ec2.NewEip(ctx, "nat-gateway-eip", &ec2.EipArgs{
			Domain: pulumi.String("vpc"),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("Network-NATGateway-EIP"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// 9b. Create NAT Gateway in public subnet (so private subnets can reach internet)
		natGateway, err := ec2.NewNatGateway(ctx, "nat-gateway", &ec2.NatGatewayArgs{
			SubnetId:     publicSubnet1.ID(),
			AllocationId: natEip.ID(),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("Network-NATGateway"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// 9c. Route private subnets' internet traffic through NAT Gateway
		_, err = ec2.NewRoute(ctx, "private-nat-route", &ec2.RouteArgs{
			RouteTableId:         privateRouteTable.ID(),
			DestinationCidrBlock: pulumi.String("0.0.0.0/0"),
			NatGatewayId:         natGateway.ID(),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// 10. Associate Public Subnets with Public Route Table
		_, err = ec2.NewRouteTableAssociation(ctx, "public-subnet-1-association", &ec2.RouteTableAssociationArgs{
			SubnetId:     publicSubnet1.ID(),
			RouteTableId: publicRouteTable.ID(),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		_, err = ec2.NewRouteTableAssociation(ctx, "public-subnet-2-association", &ec2.RouteTableAssociationArgs{
			SubnetId:     publicSubnet2.ID(),
			RouteTableId: publicRouteTable.ID(),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// 11. Associate Private Subnets with Private Route Table
		_, err = ec2.NewRouteTableAssociation(ctx, "private-subnet-1-association", &ec2.RouteTableAssociationArgs{
			SubnetId:     privateSubnet1.ID(),
			RouteTableId: privateRouteTable.ID(),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		_, err = ec2.NewRouteTableAssociation(ctx, "private-subnet-2-association", &ec2.RouteTableAssociationArgs{
			SubnetId:     privateSubnet2.ID(),
			RouteTableId: privateRouteTable.ID(),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// 12. Create Network ACL for Public Subnets
		// Allows inbound traffic on ports 80 and 443
		publicNacl, err := ec2.NewNetworkAcl(ctx, "public-nacl", &ec2.NetworkAclArgs{
			VpcId: pulumi.String(vpcId),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("Public-Subnets-NACL"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// Public NACL Inbound Rules
		// Allow HTTP (port 80) from anywhere
		_, err = ec2.NewNetworkAclRule(ctx, "public-nacl-inbound-http", &ec2.NetworkAclRuleArgs{
			NetworkAclId: publicNacl.ID(),
			RuleNumber:   pulumi.Int(100),
			Protocol:     pulumi.String("tcp"),
			RuleAction:   pulumi.String("allow"),
			CidrBlock:    pulumi.String("0.0.0.0/0"),
			FromPort:     pulumi.Int(80),
			ToPort:       pulumi.Int(80),
			Egress:       pulumi.Bool(false),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// Allow HTTPS (port 443) from anywhere
		_, err = ec2.NewNetworkAclRule(ctx, "public-nacl-inbound-https", &ec2.NetworkAclRuleArgs{
			NetworkAclId: publicNacl.ID(),
			RuleNumber:   pulumi.Int(110),
			Protocol:     pulumi.String("tcp"),
			RuleAction:   pulumi.String("allow"),
			CidrBlock:    pulumi.String("0.0.0.0/0"),
			FromPort:     pulumi.Int(443),
			ToPort:       pulumi.Int(443),
			Egress:       pulumi.Bool(false),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// Allow ephemeral ports for return traffic (1024-65535)
		_, err = ec2.NewNetworkAclRule(ctx, "public-nacl-inbound-ephemeral", &ec2.NetworkAclRuleArgs{
			NetworkAclId: publicNacl.ID(),
			RuleNumber:   pulumi.Int(120),
			Protocol:     pulumi.String("tcp"),
			RuleAction:   pulumi.String("allow"),
			CidrBlock:    pulumi.String("0.0.0.0/0"),
			FromPort:     pulumi.Int(1024),
			ToPort:       pulumi.Int(65535),
			Egress:       pulumi.Bool(false),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// Public NACL Outbound Rules
		// Allow all outbound traffic
		_, err = ec2.NewNetworkAclRule(ctx, "public-nacl-outbound-all", &ec2.NetworkAclRuleArgs{
			NetworkAclId: publicNacl.ID(),
			RuleNumber:   pulumi.Int(100),
			Protocol:     pulumi.String("-1"), // -1 means all protocols
			RuleAction:   pulumi.String("allow"),
			CidrBlock:    pulumi.String("0.0.0.0/0"),
			FromPort:     pulumi.Int(0),
			ToPort:       pulumi.Int(0),
			Egress:       pulumi.Bool(true),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// 13. Create Network ACL for Private Subnets
		// Allows inbound traffic on port 5000 from public subnets
		privateNacl, err := ec2.NewNetworkAcl(ctx, "private-nacl", &ec2.NetworkAclArgs{
			VpcId: pulumi.String(vpcId),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("Private-Subnets-NACL"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// Private NACL Inbound Rules
		// Allow port 5000 from public subnet 1 (10.8.1.0/24)
		_, err = ec2.NewNetworkAclRule(ctx, "private-nacl-inbound-5000-from-public1", &ec2.NetworkAclRuleArgs{
			NetworkAclId: privateNacl.ID(),
			RuleNumber:   pulumi.Int(100),
			Protocol:     pulumi.String("tcp"),
			RuleAction:   pulumi.String("allow"),
			CidrBlock:    pulumi.String("10.8.1.0/24"),
			FromPort:     pulumi.Int(5000),
			ToPort:       pulumi.Int(5000),
			Egress:       pulumi.Bool(false),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// Allow port 5000 from public subnet 2 (10.8.2.0/24)
		_, err = ec2.NewNetworkAclRule(ctx, "private-nacl-inbound-5000-from-public2", &ec2.NetworkAclRuleArgs{
			NetworkAclId: privateNacl.ID(),
			RuleNumber:   pulumi.Int(110),
			Protocol:     pulumi.String("tcp"),
			RuleAction:   pulumi.String("allow"),
			CidrBlock:    pulumi.String("10.8.2.0/24"),
			FromPort:     pulumi.Int(5000),
			ToPort:       pulumi.Int(5000),
			Egress:       pulumi.Bool(false),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// Allow ephemeral ports for return traffic (1024-65535)
		_, err = ec2.NewNetworkAclRule(ctx, "private-nacl-inbound-ephemeral", &ec2.NetworkAclRuleArgs{
			NetworkAclId: privateNacl.ID(),
			RuleNumber:   pulumi.Int(120),
			Protocol:     pulumi.String("tcp"),
			RuleAction:   pulumi.String("allow"),
			CidrBlock:    pulumi.String("0.0.0.0/0"),
			FromPort:     pulumi.Int(1024),
			ToPort:       pulumi.Int(65535),
			Egress:       pulumi.Bool(false),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// Private NACL Outbound Rules
		// Allow all outbound traffic
		_, err = ec2.NewNetworkAclRule(ctx, "private-nacl-outbound-all", &ec2.NetworkAclRuleArgs{
			NetworkAclId: privateNacl.ID(),
			RuleNumber:   pulumi.Int(100),
			Protocol:     pulumi.String("-1"), // -1 means all protocols
			RuleAction:   pulumi.String("allow"),
			CidrBlock:    pulumi.String("0.0.0.0/0"),
			FromPort:     pulumi.Int(0),
			ToPort:       pulumi.Int(0),
			Egress:       pulumi.Bool(true),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// 14. Associate NACLs with Subnets
		_, err = ec2.NewNetworkAclAssociation(ctx, "public-subnet-1-nacl-assoc", &ec2.NetworkAclAssociationArgs{
			NetworkAclId: publicNacl.ID(),
			SubnetId:      publicSubnet1.ID(),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		_, err = ec2.NewNetworkAclAssociation(ctx, "public-subnet-2-nacl-assoc", &ec2.NetworkAclAssociationArgs{
			NetworkAclId: publicNacl.ID(),
			SubnetId:      publicSubnet2.ID(),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		_, err = ec2.NewNetworkAclAssociation(ctx, "private-subnet-1-nacl-assoc", &ec2.NetworkAclAssociationArgs{
			NetworkAclId: privateNacl.ID(),
			SubnetId:      privateSubnet1.ID(),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		_, err = ec2.NewNetworkAclAssociation(ctx, "private-subnet-2-nacl-assoc", &ec2.NetworkAclAssociationArgs{
			NetworkAclId: privateNacl.ID(),
			SubnetId:      privateSubnet2.ID(),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// 15. Export outputs
		ctx.Export("vpcId", pulumi.String(vpcId))
		ctx.Export("internetGatewayId", igw.ID())
		ctx.Export("natGatewayId", natGateway.ID())
		ctx.Export("natGatewayEipAllocationId", natEip.ID())
		ctx.Export("publicRouteTableId", publicRouteTable.ID())
		ctx.Export("privateRouteTableId", privateRouteTable.ID())
		ctx.Export("publicSubnet1Id", publicSubnet1.ID())
		ctx.Export("publicSubnet2Id", publicSubnet2.ID())
		ctx.Export("privateSubnet1Id", privateSubnet1.ID())
		ctx.Export("privateSubnet2Id", privateSubnet2.ID())
		ctx.Export("publicNaclId", publicNacl.ID())
		ctx.Export("privateNaclId", privateNacl.ID())

		return nil
	})
}

