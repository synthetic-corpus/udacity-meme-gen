package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/dynamodb"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type DynamoResources struct {
	Table           *dynamodb.Table
	GatewayEndpoint *ec2.VpcEndpoint
}

func createDynamoResources(
	ctx *pulumi.Context,
	awsProvider *aws.Provider,
	awsRegion string,
	vpcId string,
	privateRouteTableId string,
) (*DynamoResources, error) {
	privateRouteTableIds := pulumi.StringArray{
		pulumi.String(privateRouteTableId),
	}

	dynamoTable, err := dynamodb.NewTable(ctx, "meme-generator-processing-table", &dynamodb.TableArgs{
		Name:          pulumi.String("MemeDynmoLogs"),
		TableClass:    pulumi.String("STANDARD"),
		BillingMode:   pulumi.String("PROVISIONED"),
		ReadCapacity:  pulumi.Int(5),
		WriteCapacity: pulumi.Int(5),
		HashKey:       pulumi.String("ID"),
		RangeKey:      pulumi.String("CreatedAt"),
		Attributes: dynamodb.TableAttributeArray{
			&dynamodb.TableAttributeArgs{
				Name: pulumi.String("ID"),
				Type: pulumi.String("S"),
			},
			&dynamodb.TableAttributeArgs{
				Name: pulumi.String("CreatedAt"),
				Type: pulumi.String("S"),
			},
			&dynamodb.TableAttributeArgs{
				Name: pulumi.String("SourceFile"),
				Type: pulumi.String("S"),
			},
		},
		GlobalSecondaryIndexes: dynamodb.TableGlobalSecondaryIndexArray{
			&dynamodb.TableGlobalSecondaryIndexArgs{
				Name:           pulumi.String("SourceFileIndex"),
				HashKey:        pulumi.String("SourceFile"),
				RangeKey:       pulumi.String("CreatedAt"),
				ProjectionType: pulumi.String("ALL"),
				ReadCapacity:   pulumi.Int(5),
				WriteCapacity:  pulumi.Int(5),
			},
		},
	}, pulumi.Provider(awsProvider))
	if err != nil {
		return nil, fmt.Errorf("failed to create DynamoDB table: %w", err)
	}

	dynamoGatewayEndpoint, err := ec2.NewVpcEndpoint(ctx, "dynamodb-private-gateway-endpoint", &ec2.VpcEndpointArgs{
		VpcId:           pulumi.String(vpcId),
		ServiceName:     pulumi.String(fmt.Sprintf("com.amazonaws.%s.dynamodb", awsRegion)),
		VpcEndpointType: pulumi.String("Gateway"),
		RouteTableIds:   privateRouteTableIds,
		Tags: pulumi.StringMap{
			"Name": pulumi.String("dynamodb-private-gateway-endpoint"),
		},
	}, pulumi.Provider(awsProvider))
	if err != nil {
		return nil, fmt.Errorf("failed to create DynamoDB gateway endpoint: %w", err)
	}

	return &DynamoResources{
		Table:           dynamoTable,
		GatewayEndpoint: dynamoGatewayEndpoint,
	}, nil
}
