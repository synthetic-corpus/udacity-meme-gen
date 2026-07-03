package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// PrivateServiceEndpoints collects the VPC endpoints used by workloads in private subnets.
type PrivateServiceEndpoints struct {
	SecurityGroup   *ec2.SecurityGroup
	EKSAuthEndpoint *ec2.VpcEndpoint
	S3Endpoint      *ec2.VpcEndpoint
}

func createPrivateServiceEndpoints(
	ctx *pulumi.Context,
	awsProvider *aws.Provider,
	awsRegion string,
	vpcId string,
	privateSubnetA string,
	privateSubnetB string,
) (*PrivateServiceEndpoints, error) {
	privateSubnetADetails, err := ec2.LookupSubnet(ctx, &ec2.LookupSubnetArgs{
		Id: pulumi.StringRef(privateSubnetA),
	}, pulumi.Provider(awsProvider))
	if err != nil {
		return nil, fmt.Errorf("failed to look up private subnet A for endpoint restrictions: %w", err)
	}

	privateSubnetBDetails, err := ec2.LookupSubnet(ctx, &ec2.LookupSubnetArgs{
		Id: pulumi.StringRef(privateSubnetB),
	}, pulumi.Provider(awsProvider))
	if err != nil {
		return nil, fmt.Errorf("failed to look up private subnet B for endpoint restrictions: %w", err)
	}

	endpointSecurityGroup, err := ec2.NewSecurityGroup(ctx, "private-service-endpoints-sg", &ec2.SecurityGroupArgs{
		Description: pulumi.String("Security group for private VPC endpoints used by EKS workloads"),
		VpcId:       pulumi.String(vpcId),
		Ingress: ec2.SecurityGroupIngressArray{
			&ec2.SecurityGroupIngressArgs{
				FromPort: pulumi.Int(443),
				ToPort:   pulumi.Int(443),
				Protocol: pulumi.String("tcp"),
				CidrBlocks: pulumi.StringArray{
					pulumi.String(privateSubnetADetails.CidrBlock),
					pulumi.String(privateSubnetBDetails.CidrBlock),
				},
			},
		},
		Egress: ec2.SecurityGroupEgressArray{
			&ec2.SecurityGroupEgressArgs{
				FromPort:   pulumi.Int(0),
				ToPort:     pulumi.Int(0),
				Protocol:   pulumi.String("-1"),
				CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
			},
		},
		Tags: pulumi.StringMap{
			"Name": pulumi.String("private-service-endpoints-sg"),
		},
	}, pulumi.Provider(awsProvider))
	if err != nil {
		return nil, fmt.Errorf("failed to create security group for private service endpoints: %w", err)
	}

	privateSubnetIds := pulumi.StringArray{
		pulumi.String(privateSubnetA),
		pulumi.String(privateSubnetB),
	}
	privateEndpointSecurityGroups := pulumi.StringArray{
		endpointSecurityGroup.ID(),
	}

	eksAuthEndpoint, err := ec2.NewVpcEndpoint(ctx, "eks-auth-private-endpoint", &ec2.VpcEndpointArgs{
		VpcId:             pulumi.String(vpcId),
		ServiceName:       pulumi.String(fmt.Sprintf("com.amazonaws.%s.eks-auth", awsRegion)),
		VpcEndpointType:   pulumi.String("Interface"),
		PrivateDnsEnabled: pulumi.Bool(true),
		SubnetIds:         privateSubnetIds,
		SecurityGroupIds:  privateEndpointSecurityGroups,
		Tags: pulumi.StringMap{
			"Name": pulumi.String("eks-auth-private-endpoint"),
		},
	}, pulumi.Provider(awsProvider))
	if err != nil {
		return nil, fmt.Errorf("failed to create EKS Auth VPC endpoint: %w", err)
	}

	s3Endpoint, err := ec2.NewVpcEndpoint(ctx, "s3-private-endpoint", &ec2.VpcEndpointArgs{
		VpcId:             pulumi.String(vpcId),
		ServiceName:       pulumi.String(fmt.Sprintf("com.amazonaws.%s.s3", awsRegion)),
		VpcEndpointType:   pulumi.String("Interface"),
		PrivateDnsEnabled: pulumi.Bool(true),
		SubnetIds:         privateSubnetIds,
		SecurityGroupIds:  privateEndpointSecurityGroups,
		Tags: pulumi.StringMap{
			"Name": pulumi.String("s3-private-endpoint"),
		},
	}, pulumi.Provider(awsProvider))
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 VPC endpoint: %w", err)
	}

	return &PrivateServiceEndpoints{
		SecurityGroup:   endpointSecurityGroup,
		EKSAuthEndpoint: eksAuthEndpoint,
		S3Endpoint:      s3Endpoint,
	}, nil
}
