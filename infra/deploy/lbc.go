package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/eks"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes"
	corev1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/core/v1"
	helmv3 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/helm/v3"
	metav1 "github.com/pulumi/pulumi-kubernetes/sdk/v4/go/kubernetes/meta/v1"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	lbcNamespace          = "kube-system"
	lbcServiceAccountName = "aws-load-balancer-controller"
	// Helm chart 1.14.x deploys AWS Load Balancer Controller v2.14.x.
	lbcHelmChartVersion = "1.14.0"
)

// LoadBalancerControllerResources holds the Kubernetes and AWS resources for the LBC.
type LoadBalancerControllerResources struct {
	ServiceAccount         *corev1.ServiceAccount
	PodIdentityAssociation *eks.PodIdentityAssociation
	HelmRelease            *helmv3.Release
}

func installAWSLoadBalancerController(
	ctx *pulumi.Context,
	awsProvider *aws.Provider,
	k8sProvider *kubernetes.Provider,
	cluster *eks.Cluster,
	lbcRole *iam.Role,
	vpcId string,
	awsRegion string,
	dependsOn []pulumi.Resource,
) (*LoadBalancerControllerResources, error) {
	serviceAccount, err := corev1.NewServiceAccount(ctx, "aws-load-balancer-controller-sa", &corev1.ServiceAccountArgs{
		Metadata: &metav1.ObjectMetaArgs{
			Name:      pulumi.String(lbcServiceAccountName),
			Namespace: pulumi.String(lbcNamespace),
			Labels: pulumi.StringMap{
				"app.kubernetes.io/name":      pulumi.String("aws-load-balancer-controller"),
				"app.kubernetes.io/component": pulumi.String("controller"),
			},
		},
	}, pulumi.Provider(k8sProvider), pulumi.DependsOn(dependsOn))
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS Load Balancer Controller service account: %w", err)
	}

	podIdentityAssociation, err := eks.NewPodIdentityAssociation(ctx, "aws-load-balancer-controller-pod-identity", &eks.PodIdentityAssociationArgs{
		ClusterName:    cluster.Name,
		Namespace:      pulumi.String(lbcNamespace),
		ServiceAccount: pulumi.String(lbcServiceAccountName),
		RoleArn:        lbcRole.Arn,
	},
		pulumi.Provider(awsProvider),
		pulumi.DependsOn(append(dependsOn, serviceAccount)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS Load Balancer Controller Pod Identity association: %w", err)
	}

	helmRelease, err := helmv3.NewRelease(ctx, "aws-load-balancer-controller", &helmv3.ReleaseArgs{
		Chart:     pulumi.String("aws-load-balancer-controller"),
		Version:   pulumi.String(lbcHelmChartVersion),
		Namespace: pulumi.String(lbcNamespace),
		RepositoryOpts: &helmv3.RepositoryOptsArgs{
			Repo: pulumi.String("https://aws.github.io/eks-charts"),
		},
		Values: pulumi.Map{
			"clusterName": cluster.Name,
			"region":      pulumi.String(awsRegion),
			"vpcId":       pulumi.String(vpcId),
			"serviceAccount": pulumi.Map{
				"create": pulumi.Bool(false),
				"name":   pulumi.String(lbcServiceAccountName),
			},
		},
	},
		pulumi.Provider(k8sProvider),
		pulumi.DependsOn([]pulumi.Resource{serviceAccount, podIdentityAssociation}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to install AWS Load Balancer Controller Helm release: %w", err)
	}

	return &LoadBalancerControllerResources{
		ServiceAccount:         serviceAccount,
		PodIdentityAssociation: podIdentityAssociation,
		HelmRelease:            helmRelease,
	}, nil
}
