package main

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudwatch"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/eks"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// EKSIAMRoles holds IAM roles used by the EKS cluster and node group.
type EKSIAMRoles struct {
	ClusterRole *iam.Role
	NodeRole    *iam.Role
}

func createEKSIAM(ctx *pulumi.Context, awsProvider *aws.Provider, logGroup *cloudwatch.LogGroup) (*EKSIAMRoles, error) {
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

	return &EKSIAMRoles{
		ClusterRole: eksClusterRole,
		NodeRole:    eksNodeRole,
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

func createALBControllerIAM(ctx *pulumi.Context, awsProvider *aws.Provider, cluster *eks.Cluster, awsRegion string) (*iam.Role, error) {
	currentAccount, err := aws.GetCallerIdentity(ctx, nil, nil)
	if err != nil {
		return nil, err
	}

	oidcIssuerUrl := pulumi.All(cluster.Name, pulumi.String(awsRegion)).ApplyT(func(args []interface{}) (string, error) {
		clusterName := args[0].(string)
		region := args[1].(string)
		return fmt.Sprintf("https://oidc.eks.%s.amazonaws.com/id/%s", region, clusterName), nil
	}).(pulumi.StringOutput)

	oidcProviderUrl := oidcIssuerUrl.ApplyT(func(url string) string {
		return strings.TrimPrefix(url, "https://")
	}).(pulumi.StringOutput)

	albControllerRole, err := iam.NewRole(ctx, "aws-load-balancer-controller-role", &iam.RoleArgs{
		AssumeRolePolicy: pulumi.All(oidcProviderUrl, currentAccount.AccountId).ApplyT(func(args []interface{}) (string, error) {
			providerUrl := args[0].(string)
			accountId := args[1].(string)
			policy := fmt.Sprintf(`{
				"Version": "2012-10-17",
				"Statement": [{
					"Effect": "Allow",
					"Principal": {
						"Federated": "arn:aws:iam::%s:oidc-provider/%s"
					},
					"Action": "sts:AssumeRoleWithWebIdentity",
					"Condition": {
						"StringEquals": {
							"%s:sub": "system:serviceaccount:kube-system:aws-load-balancer-controller",
							"%s:aud": "sts.amazonaws.com"
						}
					}
				}]
			}`, accountId, providerUrl, providerUrl, providerUrl)
			return policy, nil
		}).(pulumi.StringOutput),
	}, pulumi.Provider(awsProvider))
	if err != nil {
		ctx.Log.Debug(fmt.Sprintf("Error at albControllerRole: %v", err), nil)
		return nil, err
	}

	_, err = iam.NewRolePolicy(ctx, "aws-load-balancer-controller-policy", &iam.RolePolicyArgs{
		Role:   albControllerRole.Name,
		Policy: pulumi.String(albControllerPolicyJSON),
	}, pulumi.Provider(awsProvider))
	if err != nil {
		ctx.Log.Debug(fmt.Sprintf("Error at aws-load-balancer-controller-policy: %v", err), nil)
		return nil, err
	}

	return albControllerRole, nil
}

const albControllerPolicyJSON = `{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Action": [
				"iam:CreateServiceLinkedRole"
			],
			"Resource": "*",
			"Condition": {
				"StringEquals": {
					"iam:AWSServiceName": "elasticloadbalancing.amazonaws.com"
				}
			}
		},
		{
			"Effect": "Allow",
			"Action": [
				"ec2:DescribeAccountAttributes",
				"ec2:DescribeAddresses",
				"ec2:DescribeAvailabilityZones",
				"ec2:DescribeInternetGateways",
				"ec2:DescribeVpcs",
				"ec2:DescribeVpcPeeringConnections",
				"ec2:DescribeSubnets",
				"ec2:DescribeSecurityGroups",
				"ec2:DescribeInstances",
				"ec2:DescribeNetworkInterfaces",
				"ec2:DescribeTags",
				"ec2:GetCoipPoolUsage",
				"ec2:DescribeCoipPools",
				"elasticloadbalancing:DescribeLoadBalancers",
				"elasticloadbalancing:DescribeLoadBalancerAttributes",
				"elasticloadbalancing:DescribeListeners",
				"elasticloadbalancing:DescribeListenerCertificates",
				"elasticloadbalancing:DescribeSSLPolicies",
				"elasticloadbalancing:DescribeRules",
				"elasticloadbalancing:DescribeTargetGroups",
				"elasticloadbalancing:DescribeTargetGroupAttributes",
				"elasticloadbalancing:DescribeTargetHealth",
				"elasticloadbalancing:DescribeTags"
			],
			"Resource": "*"
		},
		{
			"Effect": "Allow",
			"Action": [
				"cognito-idp:DescribeUserPoolClient",
				"acm:ListCertificates",
				"acm:DescribeCertificate",
				"iam:ListServerCertificates",
				"iam:GetServerCertificate",
				"waf-regional:GetWebACL",
				"waf-regional:GetWebACLForResource",
				"waf-regional:AssociateWebACL",
				"waf-regional:DisassociateWebACL",
				"wafv2:GetWebACL",
				"wafv2:GetWebACLForResource",
				"wafv2:AssociateWebACL",
				"wafv2:DisassociateWebACL",
				"shield:GetSubscriptionState",
				"shield:DescribeProtection",
				"shield:CreateProtection",
				"shield:DeleteProtection"
			],
			"Resource": "*"
		},
		{
			"Effect": "Allow",
			"Action": [
				"ec2:AuthorizeSecurityGroupIngress",
				"ec2:RevokeSecurityGroupIngress"
			],
			"Resource": "*"
		},
		{
			"Effect": "Allow",
			"Action": [
				"ec2:CreateSecurityGroup"
			],
			"Resource": "*"
		},
		{
			"Effect": "Allow",
			"Action": [
				"ec2:CreateTags"
			],
			"Resource": "arn:aws:ec2:*:*:security-group/*",
			"Condition": {
				"StringEquals": {
					"ec2:CreateAction": "CreateSecurityGroup"
				},
				"Null": {
					"aws:RequestTag/elbv2.k8s.aws/cluster": "false"
				}
			}
		},
		{
			"Effect": "Allow",
			"Action": [
				"ec2:CreateTags",
				"ec2:DeleteTags"
			],
			"Resource": "arn:aws:ec2:*:*:security-group/*",
			"Condition": {
				"Null": {
					"aws:RequestTag/elbv2.k8s.aws/cluster": "true",
					"aws:ResourceTag/elbv2.k8s.aws/cluster": "false"
				}
			}
		},
		{
			"Effect": "Allow",
			"Action": [
				"ec2:AuthorizeSecurityGroupIngress",
				"ec2:RevokeSecurityGroupIngress",
				"ec2:DeleteSecurityGroup"
			],
			"Resource": "*",
			"Condition": {
				"Null": {
					"aws:ResourceTag/elbv2.k8s.aws/cluster": "false"
				}
			}
		},
		{
			"Effect": "Allow",
			"Action": [
				"elasticloadbalancing:CreateLoadBalancer",
				"elasticloadbalancing:CreateTargetGroup"
			],
			"Resource": "*",
			"Condition": {
				"Null": {
					"aws:RequestTag/elbv2.k8s.aws/cluster": "false"
				}
			}
		},
		{
			"Effect": "Allow",
			"Action": [
				"elasticloadbalancing:CreateListener",
				"elasticloadbalancing:DeleteListener",
				"elasticloadbalancing:CreateRule",
				"elasticloadbalancing:DeleteRule"
			],
			"Resource": "*"
		},
		{
			"Effect": "Allow",
			"Action": [
				"elasticloadbalancing:AddTags",
				"elasticloadbalancing:RemoveTags"
			],
			"Resource": [
				"arn:aws:elasticloadbalancing:*:*:targetgroup/*/*",
				"arn:aws:elasticloadbalancing:*:*:loadbalancer/net/*/*",
				"arn:aws:elasticloadbalancing:*:*:loadbalancer/app/*/*"
			],
			"Condition": {
				"Null": {
					"aws:RequestTag/elbv2.k8s.aws/cluster": "true",
					"aws:ResourceTag/elbv2.k8s.aws/cluster": "false"
				}
			}
		},
		{
			"Effect": "Allow",
			"Action": [
				"elasticloadbalancing:AddTags",
				"elasticloadbalancing:RemoveTags"
			],
			"Resource": [
				"arn:aws:elasticloadbalancing:*:*:listener/net/*/*/*",
				"arn:aws:elasticloadbalancing:*:*:listener/app/*/*/*",
				"arn:aws:elasticloadbalancing:*:*:listener-rule/net/*/*/*",
				"arn:aws:elasticloadbalancing:*:*:listener-rule/app/*/*/*"
			]
		},
		{
			"Effect": "Allow",
			"Action": [
				"elasticloadbalancing:ModifyLoadBalancerAttributes",
				"elasticloadbalancing:SetIpAddressType",
				"elasticloadbalancing:SetSecurityGroups",
				"elasticloadbalancing:SetSubnets",
				"elasticloadbalancing:DeleteLoadBalancer",
				"elasticloadbalancing:ModifyTargetGroup",
				"elasticloadbalancing:ModifyTargetGroupAttributes",
				"elasticloadbalancing:DeleteTargetGroup"
			],
			"Resource": "*",
			"Condition": {
				"Null": {
					"aws:ResourceTag/elbv2.k8s.aws/cluster": "false"
				}
			}
		},
		{
			"Effect": "Allow",
			"Action": [
				"elasticloadbalancing:AddTags"
			],
			"Resource": [
				"arn:aws:elasticloadbalancing:*:*:targetgroup/*/*",
				"arn:aws:elasticloadbalancing:*:*:loadbalancer/net/*/*",
				"arn:aws:elasticloadbalancing:*:*:loadbalancer/app/*/*"
			],
			"Condition": {
				"StringEquals": {
					"elasticloadbalancing:CreateAction": [
						"CreateTargetGroup",
						"CreateLoadBalancer"
					]
				},
				"Null": {
					"aws:RequestTag/elbv2.k8s.aws/cluster": "false"
				}
			}
		},
		{
			"Effect": "Allow",
			"Action": [
				"elasticloadbalancing:RegisterTargets",
				"elasticloadbalancing:DeregisterTargets"
			],
			"Resource": "arn:aws:elasticloadbalancing:*:*:targetgroup/*/*"
		},
		{
			"Effect": "Allow",
			"Action": [
				"elasticloadbalancing:SetWebAcl",
				"elasticloadbalancing:ModifyListener",
				"elasticloadbalancing:AddListenerCertificates",
				"elasticloadbalancing:RemoveListenerCertificates",
				"elasticloadbalancing:ModifyRule"
			],
			"Resource": "*"
		}
	]
}`
