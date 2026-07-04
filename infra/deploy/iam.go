package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudwatch"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// EKSIAMRoles holds IAM roles used by the EKS cluster, node group, and workloads.
type EKSIAMRoles struct {
	ClusterRole     *iam.Role
	NodeRole        *iam.Role
	PodIdentityRole *iam.Role
}

func createEKSIAM(ctx *pulumi.Context, awsProvider *aws.Provider, logGroup *cloudwatch.LogGroup, s3BucketName string, dynamoTableArn pulumi.StringInput) (*EKSIAMRoles, error) {
	eksClusterRole, err := iam.NewRole(ctx, "eks-cluster-role", &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Principal": {
					"Service": "eks.amazonaws.com"
				},
				"Action": "sts:AssumeRole"
			}]
		}`),
	}, pulumi.Provider(awsProvider))
	if err != nil {
		ctx.Log.Debug(fmt.Sprintf("Error at EKS Cluster Role: %v", err), nil)
		return nil, err
	}

	_, err = iam.NewRolePolicyAttachment(ctx, "eks-cluster-policy", &iam.RolePolicyAttachmentArgs{
		Role:      eksClusterRole.Name,
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"),
	}, pulumi.Provider(awsProvider))
	if err != nil {
		ctx.Log.Debug(fmt.Sprintf("Error at EKS Cluster Policy Attachment: %v", err), nil)
		return nil, err
	}

	// Required for the Kubernetes service controller to manage NLB/ENI resources.
	_, err = iam.NewRolePolicyAttachment(ctx, "eks-cluster-vpc-resource-controller", &iam.RolePolicyAttachmentArgs{
		Role:      eksClusterRole.Name,
		PolicyArn: pulumi.String("arn:aws:iam::aws:policy/AmazonEKSVPCResourceController"),
	}, pulumi.Provider(awsProvider))
	if err != nil {
		ctx.Log.Debug(fmt.Sprintf("Error at EKS VPC Resource Controller Policy Attachment: %v", err), nil)
		return nil, err
	}

	eksNodeRole, err := iam.NewRole(ctx, "eks-node-role", &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Principal": {
					"Service": "ec2.amazonaws.com"
				},
				"Action": "sts:AssumeRole"
			}]
		}`),
	}, pulumi.Provider(awsProvider))
	if err != nil {
		ctx.Log.Debug(fmt.Sprintf("Error at EKS Node Role: %v", err), nil)
		return nil, err
	}

	nodePolicies := []string{
		"arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
		"arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
		"arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
	}
	for i, policyArn := range nodePolicies {
		_, err = iam.NewRolePolicyAttachment(ctx, fmt.Sprintf("eks-node-policy-%d", i), &iam.RolePolicyAttachmentArgs{
			Role:      eksNodeRole.Name,
			PolicyArn: pulumi.String(policyArn),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at EKS Node Policy Attachment %d: %v", i, err), nil)
			return nil, err
		}
	}

	if err := attachCloudWatchLogsPolicies(ctx, awsProvider, eksClusterRole, eksNodeRole, logGroup); err != nil {
		return nil, err
	}

	if err := attachEKSNodeECRPolicy(ctx, awsProvider, eksNodeRole); err != nil {
		return nil, err
	}

	podIdentityRole, err := iam.NewRole(ctx, "meme-generator-pod-identity-role", &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Principal": {
					"Service": "pods.eks.amazonaws.com"
				},
				"Action": [
					"sts:AssumeRole",
					"sts:TagSession"
				]
			}]
		}`),
		Description: pulumi.String("IAM role assumed by the meme-generator Kubernetes service account via EKS Pod Identity"),
	}, pulumi.Provider(awsProvider))
	if err != nil {
		ctx.Log.Debug(fmt.Sprintf("Error at EKS Pod Identity Role: %v", err), nil)
		return nil, err
	}

	if err := attachPodIdentityS3Policy(ctx, awsProvider, podIdentityRole, s3BucketName); err != nil {
		return nil, err
	}
	if err := attachPodIdentityDynamoPolicy(ctx, awsProvider, podIdentityRole, dynamoTableArn); err != nil {
		return nil, err
	}

	return &EKSIAMRoles{
		ClusterRole:     eksClusterRole,
		NodeRole:        eksNodeRole,
		PodIdentityRole: podIdentityRole,
	}, nil
}

func attachCloudWatchLogsPolicies(ctx *pulumi.Context, awsProvider *aws.Provider, clusterRole, nodeRole *iam.Role, logGroup *cloudwatch.LogGroup) error {
	logsPolicy := pulumi.All(logGroup.Arn).ApplyT(func(args []interface{}) (string, error) {
		logGroupArn := args[0].(string)
		return fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Action": [
					"logs:PutLogEvents",
					"logs:CreateLogGroup",
					"logs:CreateLogStream",
					"logs:DescribeLogStreams",
					"logs:DescribeLogGroups"
				],
				"Resource": [
					"%s",
					"%s:*"
				]
			}]
		}`, logGroupArn, logGroupArn), nil
	}).(pulumi.StringOutput)

	_, err := iam.NewRolePolicy(ctx, "eks-cluster-cloudwatch-logs-policy", &iam.RolePolicyArgs{
		Role:   clusterRole.Name,
		Policy: logsPolicy,
	}, pulumi.Provider(awsProvider))
	if err != nil {
		ctx.Log.Debug(fmt.Sprintf("Error at EKS Cluster CloudWatch Logs Policy: %v", err), nil)
		return err
	}

	_, err = iam.NewRolePolicy(ctx, "eks-node-cloudwatch-logs-policy", &iam.RolePolicyArgs{
		Role:   nodeRole.Name,
		Policy: logsPolicy,
	}, pulumi.Provider(awsProvider))
	if err != nil {
		ctx.Log.Debug(fmt.Sprintf("Error at EKS Node CloudWatch Logs Policy: %v", err), nil)
		return err
	}

	return nil
}

func attachEKSNodeECRPolicy(ctx *pulumi.Context, awsProvider *aws.Provider, nodeRole *iam.Role) error {
	_, err := iam.NewRolePolicy(ctx, "eks-node-ecr-policy", &iam.RolePolicyArgs{
		Role: nodeRole.Name,
		Policy: pulumi.String(`{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Effect": "Allow",
					"Action": "ecr:GetAuthorizationToken",
					"Resource": "*"
				},
				{
					"Effect": "Allow",
					"Action": [
						"ecr:BatchCheckLayerAvailability",
						"ecr:GetDownloadUrlForLayer",
						"ecr:BatchGetImage"
					],
					"Resource": "arn:aws:ecr:*:*:repository/*"
				}
			]
		}`),
	}, pulumi.Provider(awsProvider))
	if err != nil {
		ctx.Log.Debug(fmt.Sprintf("Error at EKS Node ECR Policy: %v", err), nil)
		return err
	}
	return nil
}

func attachPodIdentityS3Policy(ctx *pulumi.Context, awsProvider *aws.Provider, podIdentityRole *iam.Role, s3BucketName string) error {
	s3Policy := pulumi.String(fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Action": "s3:*",
				"Resource": [
					"arn:aws:s3:::%s",
					"arn:aws:s3:::%s/*"
				]
			}
		]
	}`, s3BucketName, s3BucketName))

	_, err := iam.NewRolePolicy(ctx, "meme-generator-pod-identity-s3-policy", &iam.RolePolicyArgs{
		Role:   podIdentityRole.Name,
		Policy: s3Policy,
	}, pulumi.Provider(awsProvider))
	if err != nil {
		ctx.Log.Debug(fmt.Sprintf("Error at EKS Pod Identity S3 Policy: %v", err), nil)
		return err
	}

	return nil
}

func attachPodIdentityDynamoPolicy(ctx *pulumi.Context, awsProvider *aws.Provider, podIdentityRole *iam.Role, dynamoTableArn pulumi.StringInput) error {
	dynamoPolicy := pulumi.All(dynamoTableArn).ApplyT(func(args []interface{}) (string, error) {
		tableArn := args[0].(string)
		return fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Effect": "Allow",
					"Action": [
						"dynamodb:BatchGetItem",
						"dynamodb:BatchWriteItem",
						"dynamodb:ConditionCheckItem",
						"dynamodb:DeleteItem",
						"dynamodb:DescribeTable",
						"dynamodb:GetItem",
						"dynamodb:PutItem",
						"dynamodb:Query",
						"dynamodb:Scan",
						"dynamodb:UpdateItem"
					],
					"Resource": [
						"%s",
						"%s/index/*"
					]
				}
			]
		}`, tableArn, tableArn), nil
	}).(pulumi.StringOutput)

	_, err := iam.NewRolePolicy(ctx, "meme-generator-pod-identity-dynamo-policy", &iam.RolePolicyArgs{
		Role:   podIdentityRole.Name,
		Policy: dynamoPolicy,
	}, pulumi.Provider(awsProvider))
	if err != nil {
		ctx.Log.Debug(fmt.Sprintf("Error at EKS Pod Identity Dynamo Policy: %v", err), nil)
		return err
	}

	return nil
}
