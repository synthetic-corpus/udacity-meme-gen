package main

import (
	"fmt"
	"os"
	"strings"
	"os/exec"
	"encoding/json"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudwatch"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/eks"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/lb"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	networkingv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/networking/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi-docker/sdk/v4/go/docker"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Get AWS region
		awsRegion := os.Getenv("AWS_REGION")
		if awsRegion == "" {
			awsRegion = "us-west-2"
		}

		// Defines a customer provider 
		// This is fundamentally to make sure tags are consistent.
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
		// VPC  and Subnet must come from Environment variables.
		// see the Network folder for more details on how to set up network.
		vpcId := os.Getenv("TARGET_VPC_NAME")
		if vpcId == "" {
			return fmt.Errorf("TARGET_VPC_NAME environment variable is required")
		}

		// Public subnets for ALB
		publicSubnetA := os.Getenv("PUBLIC_SUBNET_A")
		publicSubnetB := os.Getenv("PUBLIC_SUBNET_B")
		if publicSubnetA == "" || publicSubnetB == "" {
			return fmt.Errorf("PUBLIC_SUBNET_A and PUBLIC_SUBNET_B environment variables are required")
		}

		// Private subnets for EKS cluster
		privateSubnetA := os.Getenv("PRIVATE_SUBNET_A")
		privateSubnetB := os.Getenv("PRIVATE_SUBNET_B")
		if privateSubnetA == "" || privateSubnetB == "" {
			return fmt.Errorf("PRIVATE_SUBNET_A and PRIVATE_SUBNET_B environment variables are required")
		}

		// ==== Docker Section ====
		// Pushes an image to docker iff the files within the folder have changed
		// since the last push.
		// Relies on Hashes to do that.
		ecrRepoUrl := os.Getenv("ECR_WEB_REPO")
		if ecrRepoUrl == "" {
			return fmt.Errorf("ECR_WEB_REPO environment variable is not set")
		}

		// Extract the server URL from the ECR repository URL (domain only, without the repo path)
		// ECR URL format: account.dkr.ecr.region.amazonaws.com/repo-name
		// Server should be just the domain part
		ecrServer := ecrRepoUrl
		if idx := strings.LastIndex(ecrRepoUrl, "/"); idx != -1 {
			ecrServer = ecrRepoUrl[:idx]
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

		// Create Docker provider for building and pushing images
		dockerProvider, err := docker.NewProvider(ctx, "docker-provider", &docker.ProviderArgs{
			// When running in Docker, the provider will use the mounted Docker socket
		})
		if err != nil {
			return err
		}

		

		// === Determines from here, should we even push? Pushes if yes ===
		currentHash, err := hashDir("/proj/src-test")
		if err != nil {
			return fmt.Errorf("failed to hash directory: %w", err)
		}
		ctx.Log.Info(fmt.Sprintf("Current directory hash: %s", currentHash), nil)
		var latestImageURL string // Will be defined later
		hashExists, err := checkIfHashExists(ctx, ecrRepoUrl, currentHash)

		if err == nil {
			if hashExists == false {
			// Build and Push the Docker image from src-test folder
			// Store hash as build argument for reference
			ctx.Log.Info("naming image: " + fmt.Sprintf("%s:%s", ecrRepoUrl, currentHash), nil)
			image, err := docker.NewImage(ctx, "meme-generator-app", &docker.ImageArgs{ // TODO use a Pipe here
				Build: &docker.DockerBuildArgs{
					Context: pulumi.String("src-test"), // Path relative to working directory /proj
					Args: pulumi.StringMap{
						"SOURCE_HASH": pulumi.String(currentHash),
					},
				},
				ImageName: pulumi.String(fmt.Sprintf("%s:%s", ecrRepoUrl, currentHash)),
				Registry: &docker.RegistryArgs{
					Server: pulumi.String(ecrServer),
				},
			}, pulumi.Provider(dockerProvider))
			if err != nil {
				return err
			}

			_, err = docker.NewTag(ctx, "my-image-latest", &docker.TagArgs{
				// this tags the image created in the above lines.
				// does not actually create a new one, or re-build. Pulumi smart.
				SourceImage: image.ImageName,
				TargetImage: pulumi.String(fmt.Sprintf("%s:latest", ecrRepoUrl)),
			})
			if err != nil {
				return err
			}

			ctx.Log.Info("Image built and pushed successfully", nil)
			}else{
				ctx.Log.Info(fmt.Sprintf("Hash unchanged (%s) - skipping build", currentHash), nil)
			}

		}else{
			// we had some kind of error in getting the image from URL
			return err
		}
		latestImageURL = fmt.Sprintf("%s:%s", ecrRepoUrl, currentHash)
		ctx.Export("imageName", pulumi.String(latestImageURL))
		ctx.Export("ecrRepoUrl", pulumi.String(ecrRepoUrl))
		ctx.Export("sourceHash", pulumi.String(currentHash))
		ctx.Export("skipped", pulumi.Bool(true))

		// TODO make this into a string, always using 'latest'
		//latestImageUrl := pulumi.Sprintf("%s:latest", ecrRepoUrl)

		// ===== EKS Cluster Setup =====
		
		// 5. Create IAM role for EKS cluster
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
			return nil
		}

		// Attach EKS cluster policy to the role
		_, err = iam.NewRolePolicyAttachment(ctx, "eks-cluster-policy", &iam.RolePolicyAttachmentArgs{
			Role:      eksClusterRole.Name,
			PolicyArn: pulumi.String("arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at EKS Cluster Policy Attachment: %v", err), nil)
			return nil
		}

		// 6. Create IAM role for EKS node group
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
			return nil
		}

		// Attach required policies for node group
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
				return nil
			}
		}

		// 6a. Create CloudWatch Log Group
		logGroup, err := cloudwatch.NewLogGroup(ctx, "meme-generator-logs", &cloudwatch.LogGroupArgs{
			Name:              pulumi.String("Meme-generator-logs"),
			RetentionInDays:    pulumi.Int(7),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at CloudWatch Log Group: %v", err), nil)
			return nil
		}

		// 6b. Create CloudWatch Logs policy for EKS cluster role
		clusterLogsPolicy, err := iam.NewRolePolicy(ctx, "eks-cluster-cloudwatch-logs-policy", &iam.RolePolicyArgs{
			Role: eksClusterRole.Name,
			Policy: pulumi.All(logGroup.Arn).ApplyT(func(args []interface{}) (string, error) {
				logGroupArn := args[0].(string)
				// Allow access to the log group and all log streams under it
				policy := fmt.Sprintf(`{
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
				}`, logGroupArn, logGroupArn)
				return policy, nil
			}).(pulumi.StringOutput),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at EKS Cluster CloudWatch Logs Policy: %v", err), nil)
			return nil
		}

		// 6c. Create CloudWatch Logs policy for EKS node group role
		nodeLogsPolicy, err := iam.NewRolePolicy(ctx, "eks-node-cloudwatch-logs-policy", &iam.RolePolicyArgs{
			Role: eksNodeRole.Name,
			Policy: pulumi.All(logGroup.Arn).ApplyT(func(args []interface{}) (string, error) {
				logGroupArn := args[0].(string)
				// Allow access to the log group and all log streams under it
				policy := fmt.Sprintf(`{
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
				}`, logGroupArn, logGroupArn)
				return policy, nil
			}).(pulumi.StringOutput),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at EKS Node CloudWatch Logs Policy: %v", err), nil)
			return nil
		}
		_ = clusterLogsPolicy
		_ = nodeLogsPolicy

		// 7. Create security group for EKS pods
		eksSecurityGroup, err := ec2.NewSecurityGroup(ctx, "eks-pod-security-group", &ec2.SecurityGroupArgs{
			Description: pulumi.String("Security group for EKS pods"),
			VpcId:       pulumi.String(vpcId),
			Ingress: ec2.SecurityGroupIngressArray{
				// Allow HTTP on port 80
				&ec2.SecurityGroupIngressArgs{
					FromPort:   pulumi.Int(80),
					ToPort:     pulumi.Int(80),
					Protocol:   pulumi.String("tcp"),
					CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
				},
				// Allow HTTPS on port 443
				&ec2.SecurityGroupIngressArgs{
					FromPort:   pulumi.Int(443),
					ToPort:     pulumi.Int(443),
					Protocol:   pulumi.String("tcp"),
					CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
				},
				// Allow application on port 5000
				&ec2.SecurityGroupIngressArgs{
					FromPort:   pulumi.Int(5000),
					ToPort:     pulumi.Int(5000),
					Protocol:   pulumi.String("tcp"),
					CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
				},
			},
			Egress: ec2.SecurityGroupEgressArray{
				// Allow all egress
				&ec2.SecurityGroupEgressArgs{
					FromPort:   pulumi.Int(0),
					ToPort:     pulumi.Int(0),
					Protocol:   pulumi.String("-1"),
					CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
				},
			},
			Tags: pulumi.StringMap{
				"Name": pulumi.String("eks-pod-security-group"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at EKS Pod Security Group: %v", err), nil)
			return nil
		}


		albSecurityGroup, err := ec2.NewSecurityGroup(ctx, "alb-security-group", &ec2.SecurityGroupArgs{
			Description: pulumi.String("Security group for Application Load Balancer"),
			VpcId:       pulumi.String(vpcId),
			Ingress: ec2.SecurityGroupIngressArray{
				// Allow HTTP from internet
				&ec2.SecurityGroupIngressArgs{
					FromPort:   pulumi.Int(80),
					ToPort:     pulumi.Int(80),
					Protocol:   pulumi.String("tcp"),
					CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
				},
				// Allow HTTPS from internet
				&ec2.SecurityGroupIngressArgs{
					FromPort:   pulumi.Int(443),
					ToPort:     pulumi.Int(443),
					Protocol:   pulumi.String("tcp"),
					CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
				},
			},
			Egress: ec2.SecurityGroupEgressArray{
				// Allow all egress
				&ec2.SecurityGroupEgressArgs{
					FromPort:   pulumi.Int(0),
					ToPort:     pulumi.Int(0),
					Protocol:   pulumi.String("-1"),
					CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
				},
			},
			Tags: pulumi.StringMap{
				"Name": pulumi.String("alb-security-group"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at ALB Security Group: %v", err), nil)
			return nil
		}

		// 7b. Allow ALB to communicate with EKS pods
		_, err = ec2.NewSecurityGroupRule(ctx, "alb-to-eks-pods", &ec2.SecurityGroupRuleArgs{
			Type:                  pulumi.String("ingress"),
			FromPort:              pulumi.Int(5000),
			ToPort:                pulumi.Int(5000),
			Protocol:              pulumi.String("tcp"),
			SourceSecurityGroupId: albSecurityGroup.ID(),
			SecurityGroupId:        eksSecurityGroup.ID(),
			Description:           pulumi.String("Allow ALB to reach EKS pods"),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at NewSecurityGroup: %s", err), nil)
			return nil
		}

		// 8. Create EKS cluster with OIDC enabled for IRSA
		// EKS cluster uses private subnets
		cluster, err := eks.NewCluster(ctx, "meme-generator-cluster", &eks.ClusterArgs{
			RoleArn: eksClusterRole.Arn,
			VpcConfig: &eks.ClusterVpcConfigArgs{
				SubnetIds: pulumi.StringArray{
					pulumi.String(privateSubnetA),
					pulumi.String(privateSubnetB),
				},
				SecurityGroupIds: pulumi.StringArray{
					eksSecurityGroup.ID(),
				},
			},
			Version: pulumi.String("1.34"),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at eks.NewCluster: %s", err), nil)
			return nil
		}

		// 9. Create EKS node group
		// Node group uses private subnets
		nodeGroup, err := eks.NewNodeGroup(ctx, "meme-generator-node-group", &eks.NodeGroupArgs{
			// TODO cluster version "1.28" seems to fail. Review all this in AWS Docs next
			ClusterName:   cluster.Name,
			NodeRoleArn:   eksNodeRole.Arn,
			SubnetIds: pulumi.StringArray{
				pulumi.String(privateSubnetA),
				pulumi.String(privateSubnetB),
			},
			ScalingConfig: &eks.NodeGroupScalingConfigArgs{
				DesiredSize: pulumi.Int(2),
				MinSize:     pulumi.Int(1),
				MaxSize:     pulumi.Int(3),
			},
			InstanceTypes: pulumi.StringArray{pulumi.String("t3.medium")},
			DiskSize:      pulumi.Int(20),
		}, pulumi.Provider(awsProvider), pulumi.DependsOn([]pulumi.Resource{cluster}))
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at eks.NewNodeGroup: %s", err), nil)
			return nil
		}

		
		kubeconfig := pulumi.All(cluster.Endpoint, cluster.CertificateAuthority.Data(), cluster.Name).ApplyT(
			func(args []interface{}) (string, error) {
				ctx.Log.Debug(fmt.Sprintf("[DEBUG] Args length: %d", len(args)), nil)
					for i, val := range args {
						ctx.Log.Debug(fmt.Sprintf("[DEBUG] Arg[%d] type: %T value: %v", i, val, val), nil)
					}

				endpoint, ok1 := args[0].(string)
				_, ok2 := args[1].(*string)
				clusterName, ok3 := args[2].(string)

				if !ok1 || !ok2 || !ok3 {
					ctx.Log.Info("Waiting for cluster values to become available. Certificate not ready", nil)
					return "", nil 
				}
		
				// Define the config as a Go Map
				config := map[string]interface{}{
					"apiVersion": "v1",
					"clusters": []map[string]interface{}{
						{
							"cluster": map[string]interface{}{
								"server":                     endpoint,
								"insecure-skip-tls-verify": true, // disabled for sanity
							},
							"name": clusterName,
						},
					},
					"contexts": []map[string]interface{}{
						{
							"context": map[string]interface{}{
								"cluster": clusterName,
								"user":    clusterName,
							},
							"name": clusterName,
						},
					},
					"current-context": clusterName,
					"kind":            "Config",
					"users": []map[string]interface{}{
						{
							"name": clusterName,
							"user": map[string]interface{}{
								"exec": map[string]interface{}{
									"apiVersion": "client.authentication.k8s.io/v1beta1",
									"command":    "aws",
									"args": []string{
										"eks", "get-token", "--cluster-name", clusterName,
									},
								},
							},
						},
					},
				}
		
				// Convert the map to a JSON string (K8s accepts JSON as Kubeconfig!)
				byteData, err := json.Marshal(config)
				if err != nil {
					ctx.Log.Debug(fmt.Sprintf("Error at eks.Kubeconfig Function: %s", err), nil)
					ctx.Log.Debug(fmt.Sprintf("Arguments were: %s", args), nil)
					return "", err
				}
				return string(byteData), nil
			},
		).(pulumi.StringOutput)

		k8sProvider, err := kubernetes.NewProvider(ctx, "k8s-provider", &kubernetes.ProviderArgs{
			Kubeconfig: kubeconfig,
		})
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at Kubconfig: %s", err), nil)
			return nil
		}

		// 11. Create Kubernetes deployment
		appLabels := pulumi.StringMap{
			"app": pulumi.String("meme-generator"),
		}
		deployment, err := appsv1.NewDeployment(ctx, "meme-generator-deployment", &appsv1.DeploymentArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:   pulumi.String("meme-generator"),
				Labels: appLabels,
			},
			Spec: &appsv1.DeploymentSpecArgs{
				Replicas: pulumi.Int(2),
				Selector: &metav1.LabelSelectorArgs{
					MatchLabels: appLabels,
				},
				Template: &corev1.PodTemplateSpecArgs{
					Metadata: &metav1.ObjectMetaArgs{
						Labels: appLabels,
					},
					Spec: &corev1.PodSpecArgs{
						Containers: corev1.ContainerArray{
							&corev1.ContainerArgs{
								Name:  pulumi.String("meme-generator"),
								ImagePullPolicy: pulumi.String("Always"),
								Image: pulumi.String(latestImageURL),
								Ports: corev1.ContainerPortArray{
									&corev1.ContainerPortArgs{
										ContainerPort: pulumi.Int(5000),
										Name:          pulumi.String("http"),
									},
								},
								Resources: &corev1.ResourceRequirementsArgs{
									Requests: pulumi.StringMap{
										"cpu":    pulumi.String("100m"),    // 0.1 CPU cores
										"memory": pulumi.String("128Mi"),   // 128 MiB memory
									},
									Limits: pulumi.StringMap{
										"cpu":    pulumi.String("500m"),    // 0.5 CPU cores max
										"memory": pulumi.String("512Mi"),   // 512 MiB memory max
									},
								},
							},
						},
					},
				},
			},
		}, pulumi.Provider(k8sProvider))
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at eks.NewDeployment: %s", err), nil)
			return nil
		}

		// 12. Create Kubernetes service (NodePort type for ALB targeting)
		service, err := corev1.NewService(ctx, "meme-generator-service", &corev1.ServiceArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:   pulumi.String("meme-generator-service"),
				Labels: appLabels,
			},
			Spec: &corev1.ServiceSpecArgs{
				Type: pulumi.String("NodePort"),
				Ports: corev1.ServicePortArray{
					&corev1.ServicePortArgs{
						Port:       pulumi.Int(80),
						TargetPort: pulumi.String("http"),
						Protocol:   pulumi.String("TCP"),
						Name:       pulumi.String("http"),
					},
				},
				Selector: appLabels,
			},
		}, pulumi.Provider(k8sProvider))
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at eks.NewService: %s", err), nil)
			return nil
		}

		// 13. Create Application Load Balancer
		// ALB uses public subnets for internet-facing access
		loadBalancer, err := lb.NewLoadBalancer(ctx, "meme-generator-alb", &lb.LoadBalancerArgs{
			Name:             pulumi.String("meme-generator-alb"),
			LoadBalancerType: pulumi.String("application"),
			Subnets: pulumi.StringArray{
				pulumi.String(publicSubnetA),
				pulumi.String(publicSubnetB),
			},
			SecurityGroups: pulumi.StringArray{
				albSecurityGroup.ID(),
			},
			Internal: pulumi.Bool(false), // false = internet-facing

			Tags: pulumi.StringMap{
				"Name": pulumi.String("meme-generator-alb"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at eks.NewLoadBalancer: %s", err), nil)
			return nil
		}

		// 14. Create Target Group
		targetGroup, err := lb.NewTargetGroup(ctx, "meme-generator-tg", &lb.TargetGroupArgs{
			Name:       pulumi.String("meme-generator-tg"),
			Port:       pulumi.Int(80),
			Protocol:   pulumi.String("HTTP"),
			VpcId:      pulumi.String(vpcId),
			TargetType: pulumi.String("ip"),
			HealthCheck: &lb.TargetGroupHealthCheckArgs{
				Enabled:            pulumi.Bool(true),
				HealthyThreshold:   pulumi.Int(2),
				UnhealthyThreshold: pulumi.Int(2),
				Timeout:            pulumi.Int(5),
				Interval:           pulumi.Int(30),
				Path:               pulumi.String("/health"),
				Protocol:           pulumi.String("HTTP"),
				Port:               pulumi.String("traffic-port"),
				Matcher:            pulumi.String("200"),
			},
			Tags: pulumi.StringMap{
				"Name": pulumi.String("meme-generator-tg"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at NewTargetGroup: %s", err), nil)
			return nil
		}

		// 15. Create Load Balancer Listener
		listener, err := lb.NewListener(ctx, "meme-generator-listener", &lb.ListenerArgs{
			LoadBalancerArn: loadBalancer.Arn,
			Port:            pulumi.Int(80),
			Protocol:        pulumi.String("HTTP"),
			DefaultActions: lb.ListenerDefaultActionArray{
				&lb.ListenerDefaultActionArgs{
					Type:           pulumi.String("forward"),
					TargetGroupArn: targetGroup.Arn,
				},
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at lb.NewListener: %s", err), nil)
			return nil
		}
		_ = listener

		// 16. Get current AWS account ID and construct OIDC provider URL
		currentAccount, err := aws.GetCallerIdentity(ctx, nil, nil)
		if err != nil {
			return err
		}

		// Construct OIDC issuer URL from cluster (EKS automatically creates this)
		// Format: https://oidc.eks.<region>.amazonaws.com/id/<OIDC_ID>
		oidcIssuerUrl := pulumi.All(cluster.Name, awsRegion).ApplyT(func(args []interface{}) (string, error) {
			clusterName := args[0].(string)
			region := args[1].(string)
			// For now, we'll use a pattern. In production, get the actual OIDC ID from cluster
			// This is a simplified approach - the OIDC provider is created automatically by EKS
			return fmt.Sprintf("https://oidc.eks.%s.amazonaws.com/id/%s", region, clusterName), nil
		}).(pulumi.StringOutput)

		// Extract OIDC provider URL (without https://)
		oidcProviderUrl := oidcIssuerUrl.ApplyT(func(url string) string {
			return strings.TrimPrefix(url, "https://")
		}).(pulumi.StringOutput)

		// 17. Create IAM role for AWS Load Balancer Controller with IRSA
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
			ctx.Log.Debug(fmt.Sprintf("Error at albControllerRole: %s", err), nil)
			return nil
		}

		// 17. Attach AWS Load Balancer Controller policy
		albControllerPolicy := `{
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

		_, err = iam.NewRolePolicy(ctx, "aws-load-balancer-controller-policy", &iam.RolePolicyArgs{
			Role:   albControllerRole.Name,
			Policy: pulumi.String(albControllerPolicy),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at aws-load-balancer-controller-policy: %s", err), nil)
			return nil
		}

		// 18. Create Kubernetes ServiceAccount for AWS Load Balancer Controller
		serviceAccount, err := corev1.NewServiceAccount(ctx, "aws-load-balancer-controller-sa", &corev1.ServiceAccountArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String("aws-load-balancer-controller"),
				Namespace: pulumi.String("kube-system"),
				Annotations: pulumi.StringMap{
					"eks.amazonaws.com/role-arn": albControllerRole.Arn,
				},
			},
		}, pulumi.Provider(k8sProvider))
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at aws-load-balancer-controller-sa: %s", err), nil)
			return nil
		}
		_ = serviceAccount

		// 19. Create Kubernetes Ingress resource (AWS Load Balancer Controller will manage the ALB)
		ingress, err := networkingv1.NewIngress(ctx, "meme-generator-ingress", &networkingv1.IngressArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:      pulumi.String("meme-generator-ingress"),
				Namespace: pulumi.String("default"),
				Annotations: pulumi.StringMap{
					"kubernetes.io/ingress.class":                pulumi.String("alb"),
					"alb.ingress.kubernetes.io/scheme":           pulumi.String("internet-facing"),
					"alb.ingress.kubernetes.io/target-type":     pulumi.String("ip"),
					"alb.ingress.kubernetes.io/listen-ports":    pulumi.String("[{\"HTTP\": 80}]"),
					"alb.ingress.kubernetes.io/healthcheck-path": pulumi.String("/health"),
					"alb.ingress.kubernetes.io/healthcheck-protocol": pulumi.String("HTTP"),
				},
			},
			Spec: &networkingv1.IngressSpecArgs{
				Rules: networkingv1.IngressRuleArray{
					&networkingv1.IngressRuleArgs{
						Http: &networkingv1.HTTPIngressRuleValueArgs{
							Paths: networkingv1.HTTPIngressPathArray{
								&networkingv1.HTTPIngressPathArgs{
									Path:     pulumi.String("/"),
									PathType: pulumi.String("Prefix"),
									Backend: &networkingv1.IngressBackendArgs{
										Service: &networkingv1.IngressServiceBackendArgs{
											Name: pulumi.String("meme-generator-service"),
											Port: &networkingv1.ServiceBackendPortArgs{
												Number: pulumi.Int(80),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}, pulumi.Provider(k8sProvider), pulumi.DependsOn([]pulumi.Resource{service, serviceAccount}))
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at meme-generator-ingress: %s", err), nil)
			return nil
		}

		// Export the Subnet IDs, ECR Repository URL, EKS cluster info, CloudWatch Log Group, ALB info, and Ingress
		ctx.Export("publicSubnetA", pulumi.String(publicSubnetA))
		ctx.Export("publicSubnetB", pulumi.String(publicSubnetB))
		ctx.Export("privateSubnetA", pulumi.String(privateSubnetA))
		ctx.Export("privateSubnetB", pulumi.String(privateSubnetB))
		ctx.Export("fullImageName", pulumi.String(latestImageURL))
		ctx.Export("clusterName", cluster.Name)
		ctx.Export("clusterEndpoint", cluster.Endpoint)
		ctx.Export("nodeGroupName", nodeGroup.NodeGroupName)
		ctx.Export("deploymentName", deployment.Metadata.Name())
		ctx.Export("serviceName", service.Metadata.Name())
		ctx.Export("logGroupName", logGroup.Name)
		ctx.Export("ingressName", ingress.Metadata.Name())
		ctx.Export("albControllerRoleArn", albControllerRole.Arn)
		// Note: The ALB will be created by AWS Load Balancer Controller when the Ingress is processed
		// The manually created ALB above is kept for reference but won't be used with Ingress
		ctx.Export("loadBalancerArn", loadBalancer.Arn)
		ctx.Export("loadBalancerDnsName", loadBalancer.DnsName)
		ctx.Export("loadBalancerUrl", loadBalancer.DnsName.ApplyT(func(dns string) string {
			return fmt.Sprintf("http://%s", dns)
		}).(pulumi.StringOutput))
		return nil
	})
}
