package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/eks"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	appsv1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/apps/v1"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
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

		// Create the first subnet in us-west-2a
		subnetA, err := ec2.NewSubnet(ctx, "pulumi-managed-subnet-a", &ec2.SubnetArgs{
			VpcId:            pulumi.String(vpcId),
			CidrBlock:        pulumi.String("10.8.101.0/24"),
			AvailabilityZone: pulumi.String("us-west-2a"),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("Pulumi-Subnet-A"),
			},
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// Create the second subnet in us-west-2b (different AZ)
		subnetB, err := ec2.NewSubnet(ctx, "pulumi-managed-subnet-b", &ec2.SubnetArgs{
			VpcId:            pulumi.String(vpcId),
			CidrBlock:        pulumi.String("10.8.102.0/24"), // Different CIDR block
			AvailabilityZone: pulumi.String("us-west-2b"),
			Tags: pulumi.StringMap{
				"Name": pulumi.String("Pulumi-Subnet-B"),
			},
		}, pulumi.Provider(awsProvider))
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
			return err
		}

		// Attach EKS cluster policy to the role
		_, err = iam.NewRolePolicyAttachment(ctx, "eks-cluster-policy", &iam.RolePolicyAttachmentArgs{
			Role:      eksClusterRole.Name,
			PolicyArn: pulumi.String("arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
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
			return err
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
				return err
			}
		}

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
			return err
		}

		// 8. Create EKS cluster
		cluster, err := eks.NewCluster(ctx, "meme-generator-cluster", &eks.ClusterArgs{
			RoleArn: eksClusterRole.Arn,
			VpcConfig: &eks.ClusterVpcConfigArgs{
				SubnetIds: pulumi.StringArray{
					subnetA.ID(),
					subnetB.ID(),
				},
				SecurityGroupIds: pulumi.StringArray{
					eksSecurityGroup.ID(),
				},
			},
			Version: pulumi.String("1.28"),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return err
		}

		// 9. Create EKS node group
		nodeGroup, err := eks.NewNodeGroup(ctx, "meme-generator-node-group", &eks.NodeGroupArgs{
			ClusterName:   cluster.Name,
			NodeRoleArn:   eksNodeRole.Arn,
			SubnetIds: pulumi.StringArray{
				subnetA.ID(),
				subnetB.ID(),
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
			return err
		}

		// 10. Create Kubernetes provider
		// Construct kubeconfig from cluster details
		awsRegion := os.Getenv("AWS_REGION")
		if awsRegion == "" {
			awsRegion = "us-west-2"
		}
		
		kubeconfig := pulumi.All(cluster.Endpoint, cluster.CertificateAuthority, cluster.Name).ApplyT(func(args []interface{}) (string, error) {
			endpoint := args[0].(string)
			ca := args[1].(eks.ClusterCertificateAuthority)
			clusterName := args[2].(string)
			caData := ca.Data
			
			// Construct kubeconfig YAML
			kubeconfigYaml := fmt.Sprintf(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: %s
    server: %s
  name: %s
contexts:
- context:
    cluster: %s
    user: %s
  name: %s
current-context: %s
kind: Config
users:
- name: %s
  user:
    exec:
      apiVersion: client.authentication.k8s.io/v1beta1
      command: aws
      args:
        - eks
        - get-token
        - --cluster-name
        - %s
        - --region
        - %s
`, caData, endpoint, clusterName, clusterName, clusterName, clusterName, clusterName, clusterName, clusterName, awsRegion)
			return kubeconfigYaml, nil
		}).(pulumi.StringOutput)

		k8sProvider, err := kubernetes.NewProvider(ctx, "k8s-provider", &kubernetes.ProviderArgs{
			Kubeconfig: kubeconfig,
		})
		if err != nil {
			return err
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
								Image: image.ImageName,
								Ports: corev1.ContainerPortArray{
									&corev1.ContainerPortArgs{
										ContainerPort: pulumi.Int(5000),
										Name:          pulumi.String("http"),
									},
								},
							},
						},
					},
				},
			},
		}, pulumi.Provider(k8sProvider))
		if err != nil {
			return err
		}

		// 12. Create Kubernetes service
		service, err := corev1.NewService(ctx, "meme-generator-service", &corev1.ServiceArgs{
			Metadata: &metav1.ObjectMetaArgs{
				Name:   pulumi.String("meme-generator-service"),
				Labels: appLabels,
			},
			Spec: &corev1.ServiceSpecArgs{
				Type: pulumi.String("LoadBalancer"),
				Ports: corev1.ServicePortArray{
					&corev1.ServicePortArgs{
						Port:       pulumi.Int(80),
						TargetPort: pulumi.String("http"),
						Protocol:   pulumi.String("TCP"),
						Name:       pulumi.String("http"),
					},
					&corev1.ServicePortArgs{
						Port:       pulumi.Int(443),
						TargetPort: pulumi.String("http"),
						Protocol:   pulumi.String("TCP"),
						Name:       pulumi.String("https"),
					},
					&corev1.ServicePortArgs{
						Port:       pulumi.Int(5000),
						TargetPort: pulumi.String("http"),
						Protocol:   pulumi.String("TCP"),
						Name:       pulumi.String("app"),
					},
				},
				Selector: appLabels,
			},
		}, pulumi.Provider(k8sProvider))
		if err != nil {
			return err
		}

		// Export the Subnet IDs, ECR Repository URL, and EKS cluster info
		ctx.Export("subnetIdA", subnetA.ID())
		ctx.Export("subnetIdB", subnetB.ID())
		ctx.Export("fullImageName", image.ImageName)
		ctx.Export("clusterName", cluster.Name)
		ctx.Export("clusterEndpoint", cluster.Endpoint)
		ctx.Export("nodeGroupName", nodeGroup.NodeGroupName)
		ctx.Export("deploymentName", deployment.Metadata.Name())
		ctx.Export("serviceName", service.Metadata.Name())
		return nil
	})
}
