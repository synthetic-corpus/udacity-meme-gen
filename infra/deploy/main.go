package main

// TODO Review all of the Ports. Docker will listen on 5000, does anything else talk to on that port? Review All Security groups too.
import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudwatch"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/eks"
	"github.com/pulumi/pulumi-docker/sdk/v4/go/docker"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	k8syaml "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/yaml"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	memeGeneratorNamespace          = "default"
	memeGeneratorServiceAccountName = "meme-generator-sa"
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
					"Contact":     pulumi.String("Joel@joelgonzaga.com"),
					"CreatedBy":   pulumi.String("Udacity Meme Generator - Pulumi Deployment"),
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

		// Public subnets: NLB subnet discovery (kubernetes.io/role/elb); nodes stay in private subnets.
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

		// Must match kubernetes.io/cluster/* tags on subnets in the network stack.
		eksClusterName := os.Getenv("EKS_CLUSTER_NAME")
		if eksClusterName == "" {
			eksClusterName = "meme-generator-cluster"
		}

		// ==== Docker Section ====
		// Pushes an image to docker iff the files within the folder have changed
		// since the last push.
		// Relies on Hashes to do that.
		ecrRepoUrl := os.Getenv("ECR_WEB_REPO")
		if ecrRepoUrl == "" {
			return fmt.Errorf("ECR_WEB_REPO environment variable is not set")
		}

		s3BucketName := os.Getenv("MY_S3_BUCKET")
		if s3BucketName == "" {
			return fmt.Errorf("MY_S3_BUCKET environment variable is required")
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
				ctx.Log.Info("naming image: "+fmt.Sprintf("%s:%s", ecrRepoUrl, currentHash), nil)
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
			} else {
				ctx.Log.Info(fmt.Sprintf("Hash unchanged (%s) - skipping build", currentHash), nil)
			}

		} else {
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

		logGroup, err := cloudwatch.NewLogGroup(ctx, "meme-generator-logs", &cloudwatch.LogGroupArgs{
			Name:            pulumi.String("Meme-generator-logs"),
			RetentionInDays: pulumi.Int(7),
		}, pulumi.Provider(awsProvider))
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at CloudWatch Log Group: %v", err), nil)
			return nil
		}

		eksIAM, err := createEKSIAM(ctx, awsProvider, logGroup, s3BucketName)
		if err != nil {
			return nil
		}
		eksClusterRole := eksIAM.ClusterRole
		eksNodeRole := eksIAM.NodeRole
		memeGeneratorPodIdentityRole := eksIAM.PodIdentityRole

		// Security group for EKS pods / nodes (NLB IP targets reach pods on port 5000).
		eksSecurityGroup, err := ec2.NewSecurityGroup(ctx, "eks-pod-security-group", &ec2.SecurityGroupArgs{
			Description: pulumi.String("Security group for EKS pods"),
			VpcId:       pulumi.String(vpcId),
			Ingress: ec2.SecurityGroupIngressArray{
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

		privateServiceEndpoints, err := createPrivateServiceEndpoints(
			ctx,
			awsProvider,
			awsRegion,
			vpcId,
			privateSubnetA,
			privateSubnetB,
		)
		if err != nil {
			return fmt.Errorf("failed to create private service endpoints: %w", err)
		}

		// EKS cluster spans public + private subnets so the AWS cloud controller can place
		// internet-facing NLBs in public subnets while nodes remain in private subnets.
		cluster, err := eks.NewCluster(ctx, "meme-generator-cluster", &eks.ClusterArgs{
			Name:    pulumi.String(eksClusterName),
			RoleArn: eksClusterRole.Arn,
			VpcConfig: &eks.ClusterVpcConfigArgs{
				SubnetIds: pulumi.StringArray{
					pulumi.String(publicSubnetA),
					pulumi.String(publicSubnetB),
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

		// Nodes need the cluster SG plus eks-pod-security-group for NLB→pod traffic on port 5000.
		nodeGroupSgIds := pulumi.All(cluster.VpcConfig.ClusterSecurityGroupId(), eksSecurityGroup.ID()).ApplyT(func(args []interface{}) (pulumi.StringArray, error) {
			var sgs pulumi.StringArray
			if args[0] != nil {
				if p, ok := args[0].(*string); ok && p != nil && *p != "" {
					sgs = append(sgs, pulumi.String(*p))
				}
			}
			// eksSecurityGroup.ID() returns pulumi.IDOutput; resolved value can be pulumi.ID (string alias)
			var podSG string
			switch v := args[1].(type) {
			case string:
				podSG = v
			case pulumi.ID:
				podSG = string(v)
			default:
				podSG = fmt.Sprintf("%v", v)
			}
			sgs = append(sgs, pulumi.String(podSG))
			return sgs, nil
		}).(pulumi.StringArrayOutput)

		nodeLaunchTemplate, err := ec2.NewLaunchTemplate(ctx, "eks-node-launch-template", &ec2.LaunchTemplateArgs{
			NamePrefix:  pulumi.String("eks-meme-"),
			Description: pulumi.String("Launch template for EKS node group with custom provider"),

			// Move Disk Configuration here
			BlockDeviceMappings: ec2.LaunchTemplateBlockDeviceMappingArray{
				&ec2.LaunchTemplateBlockDeviceMappingArgs{
					DeviceName: pulumi.String("/dev/xvda"), // Default for EKS-optimized AMI
					Ebs: &ec2.LaunchTemplateBlockDeviceMappingEbsArgs{
						VolumeSize: pulumi.Int(20),
						VolumeType: pulumi.String("gp3"),
					},
				},
			},

			// Move Instance Type here for consistency
			InstanceType: pulumi.String("t3.medium"),

			VpcSecurityGroupIds: nodeGroupSgIds,
			TagSpecifications: ec2.LaunchTemplateTagSpecificationArray{
				&ec2.LaunchTemplateTagSpecificationArgs{
					ResourceType: pulumi.String("instance"),
					Tags: pulumi.StringMap{
						"Name": pulumi.String("meme-generator-eks-node"),
					},
				},
			},
		},
			pulumi.Provider(awsProvider), // <--- Custom provider applied here
			pulumi.DependsOn([]pulumi.Resource{cluster}),
		)
		if err != nil {
			ctx.Log.Debug(fmt.Sprintf("Error at ec2.NewLaunchTemplate: %s", err), nil)
			return nil
		}

		// Pods on these nodes inherit the node's security groups → ingress 5000, egress all.
		nodeGroup, err := eks.NewNodeGroup(ctx, "meme-generator-node-group", &eks.NodeGroupArgs{
			ClusterName: cluster.Name,
			NodeRoleArn: eksNodeRole.Arn,
			SubnetIds: pulumi.StringArray{
				pulumi.String(privateSubnetA),
				pulumi.String(privateSubnetB),
			},
			// ---- Reference the Launch Template here ----
			LaunchTemplate: &eks.NodeGroupLaunchTemplateArgs{
				Id:      nodeLaunchTemplate.ID(),  // Grabs the ID output from your launch template resource
				Version: pulumi.String("$Latest"), // Or specify a hardcoded version like "1"
			},
			ScalingConfig: &eks.NodeGroupScalingConfigArgs{
				DesiredSize: pulumi.Int(1), // Locked to exactly 1 node as planned
				MinSize:     pulumi.Int(1),
				MaxSize:     pulumi.Int(1),
			},
		},
			pulumi.Provider(awsProvider),
			// Ensure the cluster and the launch template are fully provisioned first
			pulumi.DependsOn([]pulumi.Resource{cluster, nodeLaunchTemplate}),
		)
		if err != nil {
			// Make sure to return the actual error here instead of nil so Pulumi knows it failed!
			return fmt.Errorf("failed to create EKS node group: %w", err)
		}

		podIdentityAgentAddon, err := eks.NewAddon(ctx, "eks-pod-identity-agent", &eks.AddonArgs{
			ClusterName:              cluster.Name,
			AddonName:                pulumi.String("eks-pod-identity-agent"),
			ResolveConflictsOnCreate: pulumi.String("OVERWRITE"),
			ResolveConflictsOnUpdate: pulumi.String("OVERWRITE"),
		},
			pulumi.Provider(awsProvider),
			pulumi.DependsOn([]pulumi.Resource{nodeGroup, privateServiceEndpoints.EKSAuthEndpoint}),
		)
		if err != nil {
			return fmt.Errorf("failed to create EKS Pod Identity agent addon: %w", err)
		}

		podIdentityAssociation, err := eks.NewPodIdentityAssociation(ctx, "meme-generator-pod-identity-association", &eks.PodIdentityAssociationArgs{
			ClusterName:    cluster.Name,
			Namespace:      pulumi.String(memeGeneratorNamespace),
			ServiceAccount: pulumi.String(memeGeneratorServiceAccountName),
			RoleArn:        memeGeneratorPodIdentityRole.Arn,
		},
			pulumi.Provider(awsProvider),
			pulumi.DependsOn([]pulumi.Resource{podIdentityAgentAddon}),
		)
		if err != nil {
			return fmt.Errorf("failed to create EKS Pod Identity association: %w", err)
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
								"server":                   endpoint,
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
		// Deploy k8s/*.yaml; inject the ECR image URL into the Deployment via transformation.
		_, err = k8syaml.NewConfigGroup(ctx, "meme-app-manifests", &k8syaml.ConfigGroupArgs{
			Files: []string{
				"k8s/serviceaccount.yaml",
				"k8s/deployment.yaml",
				"k8s/service.yaml",
				"k8s/hpa.yaml",
			},
			Transformations: []k8syaml.Transformation{
				deploymentImageTransform(latestImageURL),
				loadBalancerSubnetTransform(publicSubnetA, publicSubnetB),
			},
		},
			pulumi.Provider(k8sProvider),
			pulumi.DependsOn([]pulumi.Resource{nodeGroup, podIdentityAgentAddon, podIdentityAssociation, privateServiceEndpoints.S3Endpoint}),
		)
		if err != nil {
			return fmt.Errorf("failed to apply kubernetes manifests: %w", err)
		}

		// Export subnet IDs and EKS cluster info
		ctx.Export("publicSubnetA", pulumi.String(publicSubnetA))
		ctx.Export("publicSubnetB", pulumi.String(publicSubnetB))
		ctx.Export("privateSubnetA", pulumi.String(privateSubnetA))
		ctx.Export("privateSubnetB", pulumi.String(privateSubnetB))
		ctx.Export("fullImageName", pulumi.String(latestImageURL))
		ctx.Export("clusterName", cluster.Name)
		ctx.Export("clusterEndpoint", cluster.Endpoint)
		ctx.Export("nodeGroupName", nodeGroup.NodeGroupName)
		ctx.Export("logGroupName", logGroup.Name)
		ctx.Export("loadBalancerServiceName", pulumi.String("meme-generator-service"))
		ctx.Export("podIdentityRoleArn", memeGeneratorPodIdentityRole.Arn)
		ctx.Export("privateEndpointSecurityGroupId", privateServiceEndpoints.SecurityGroup.ID())
		ctx.Export("eksAuthEndpointId", privateServiceEndpoints.EKSAuthEndpoint.ID())
		ctx.Export("s3EndpointId", privateServiceEndpoints.S3Endpoint.ID())
		ctx.Export("podIdentityBucketName", pulumi.String(s3BucketName))
		return nil
	})
}
