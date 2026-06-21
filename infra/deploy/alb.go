package main

import (
	"fmt"
	"os"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/lb"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

// ALBResources holds the public Application Load Balancer and its pod target group.
type ALBResources struct {
	LoadBalancer *lb.LoadBalancer
	TargetGroup  *lb.TargetGroup
}

// createALBSecurityGroups creates the ALB security group and allows it to reach EKS pods on port 5000.
func createALBSecurityGroups(ctx *pulumi.Context, awsProvider *aws.Provider, vpcId string, eksSecurityGroup *ec2.SecurityGroup) (*ec2.SecurityGroup, error) {
	albSecurityGroup, err := ec2.NewSecurityGroup(ctx, "alb-security-group", &ec2.SecurityGroupArgs{
		Description: pulumi.String("Security group for Application Load Balancer"),
		VpcId:       pulumi.String(vpcId),
		Ingress: ec2.SecurityGroupIngressArray{
			&ec2.SecurityGroupIngressArgs{
				FromPort:   pulumi.Int(80),
				ToPort:     pulumi.Int(80),
				Protocol:   pulumi.String("tcp"),
				CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
			},
			&ec2.SecurityGroupIngressArgs{
				FromPort:   pulumi.Int(443),
				ToPort:     pulumi.Int(443),
				Protocol:   pulumi.String("tcp"),
				CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
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
			"Name": pulumi.String("alb-security-group"),
		},
	}, pulumi.Provider(awsProvider))
	if err != nil {
		ctx.Log.Debug(fmt.Sprintf("Error at ALB Security Group: %v", err), nil)
		return nil, err
	}

	_, err = ec2.NewSecurityGroupRule(ctx, "alb-to-eks-pods", &ec2.SecurityGroupRuleArgs{
		Type:                  pulumi.String("ingress"),
		FromPort:              pulumi.Int(5000),
		ToPort:                pulumi.Int(5000),
		Protocol:              pulumi.String("tcp"),
		SourceSecurityGroupId: albSecurityGroup.ID(),
		SecurityGroupId:       eksSecurityGroup.ID(),
		Description:           pulumi.String("Allow ALB to reach EKS pods"),
	}, pulumi.Provider(awsProvider))
	if err != nil {
		ctx.Log.Debug(fmt.Sprintf("Error at alb-to-eks-pods security group rule: %v", err), nil)
		return nil, err
	}

	return albSecurityGroup, nil
}

// createApplicationLoadBalancer creates an internet-facing ALB with HTTP/HTTPS listeners
// forwarding to an IP-mode target group on port 5000 (pod port).
func createApplicationLoadBalancer(
	ctx *pulumi.Context,
	awsProvider *aws.Provider,
	vpcId string,
	publicSubnetA string,
	publicSubnetB string,
	albSecurityGroup *ec2.SecurityGroup,
) (*ALBResources, error) {
	loadBalancer, err := lb.NewLoadBalancer(ctx, "meme-generator-alb", &lb.LoadBalancerArgs{
		Name:             pulumi.String("meme-generator-alb"),
		LoadBalancerType: pulumi.String("application"),
		Internal:         pulumi.Bool(false),
		Subnets: pulumi.StringArray{
			pulumi.String(publicSubnetA),
			pulumi.String(publicSubnetB),
		},
		SecurityGroups: pulumi.StringArray{
			albSecurityGroup.ID(),
		},
		Tags: pulumi.StringMap{
			"Name": pulumi.String("meme-generator-alb"),
		},
	}, pulumi.Provider(awsProvider))
	if err != nil {
		return nil, fmt.Errorf("create ALB: %w", err)
	}

	targetGroup, err := lb.NewTargetGroup(ctx, "meme-generator-tg", &lb.TargetGroupArgs{
		Name:       pulumi.String("meme-generator-tg"),
		Port:       pulumi.Int(5000),
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
		return nil, fmt.Errorf("create target group: %w", err)
	}

	forwardAction := lb.ListenerDefaultActionArray{
		&lb.ListenerDefaultActionArgs{
			Type:           pulumi.String("forward"),
			TargetGroupArn: targetGroup.Arn,
		},
	}

	_, err = lb.NewListener(ctx, "meme-generator-http-listener", &lb.ListenerArgs{
		LoadBalancerArn: loadBalancer.Arn,
		Port:            pulumi.Int(80),
		Protocol:        pulumi.String("HTTP"),
		DefaultActions:  forwardAction,
	}, pulumi.Provider(awsProvider))
	if err != nil {
		return nil, fmt.Errorf("create HTTP listener: %w", err)
	}

	certArn := os.Getenv("ALB_CERT_ARN")
	if certArn != "" {
		_, err = lb.NewListener(ctx, "meme-generator-https-listener", &lb.ListenerArgs{
			LoadBalancerArn: loadBalancer.Arn,
			Port:            pulumi.Int(443),
			Protocol:        pulumi.String("HTTPS"),
			SslPolicy:       pulumi.String("ELBSecurityPolicy-TLS13-1-2-2021-06"),
			CertificateArn:  pulumi.String(certArn),
			DefaultActions:  forwardAction,
		}, pulumi.Provider(awsProvider))
		if err != nil {
			return nil, fmt.Errorf("create HTTPS listener: %w", err)
		}
	} else {
		ctx.Log.Warn("ALB_CERT_ARN not set; HTTPS listener on port 443 was skipped. Set ALB_CERT_ARN to an ACM certificate ARN to enable HTTPS.", nil)
	}

	return &ALBResources{
		LoadBalancer: loadBalancer,
		TargetGroup:  targetGroup,
	}, nil
}
