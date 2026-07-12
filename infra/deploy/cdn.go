package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudfront"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type CDNResources struct {
	CachePolicy         *cloudfront.CachePolicy
	OriginAccessControl *cloudfront.OriginAccessControl
	Distribution        *cloudfront.Distribution
	BucketPolicy        *s3.BucketPolicy
}

func createCDNResources(
	ctx *pulumi.Context,
	awsProvider *aws.Provider,
	awsRegion string,
	s3BucketName string,
) (*CDNResources, error) {
	cachePolicy, err := cloudfront.NewCachePolicy(ctx, "meme-generator-cache-policy", &cloudfront.CachePolicyArgs{
		Name:       pulumi.String("myCacheName"),
		DefaultTtl: pulumi.Int(9000),
		MaxTtl:     pulumi.Int(18000),
		MinTtl:     pulumi.Int(60),
		ParametersInCacheKeyAndForwardedToOrigin: &cloudfront.CachePolicyParametersInCacheKeyAndForwardedToOriginArgs{
			CookiesConfig: &cloudfront.CachePolicyParametersInCacheKeyAndForwardedToOriginCookiesConfigArgs{
				CookieBehavior: pulumi.String("none"),
			},
			EnableAcceptEncodingGzip: pulumi.Bool(false),
			HeadersConfig: &cloudfront.CachePolicyParametersInCacheKeyAndForwardedToOriginHeadersConfigArgs{
				HeaderBehavior: pulumi.String("whitelist"),
				Headers: &cloudfront.CachePolicyParametersInCacheKeyAndForwardedToOriginHeadersConfigHeadersArgs{
					Items: pulumi.StringArray{
						pulumi.String("Authorization"),
					},
				},
			},
			QueryStringsConfig: &cloudfront.CachePolicyParametersInCacheKeyAndForwardedToOriginQueryStringsConfigArgs{
				QueryStringBehavior: pulumi.String("none"),
			},
		},
	}, pulumi.Provider(awsProvider))
	if err != nil {
		return nil, fmt.Errorf("failed to create CloudFront cache policy: %w", err)
	}

	originAccessControl, err := cloudfront.NewOriginAccessControl(ctx, "meme-generator-origin-access-control", &cloudfront.OriginAccessControlArgs{
		Description:                   pulumi.String("A OAC for the CloudFront distribution. Controls S3 permissions."),
		Name:                          pulumi.String("My Simple OAC"),
		OriginAccessControlOriginType: pulumi.String("s3"),
		SigningBehavior:               pulumi.String("no-override"),
		SigningProtocol:               pulumi.String("sigv4"),
	}, pulumi.Provider(awsProvider))
	if err != nil {
		return nil, fmt.Errorf("failed to create CloudFront origin access control: %w", err)
	}

	distribution, err := cloudfront.NewDistribution(ctx, "meme-generator-cdn", &cloudfront.DistributionArgs{
		Comment:     pulumi.String("CloudFront Distribution for S3 Bucket"),
		Enabled:     pulumi.Bool(true),
		HttpVersion: pulumi.String("http2"),
		PriceClass:  pulumi.String("PriceClass_100"),
		Origins: cloudfront.DistributionOriginArray{
			&cloudfront.DistributionOriginArgs{
				OriginId:              pulumi.String("S3Origin"),
				DomainName:            pulumi.String(fmt.Sprintf("%s.s3.%s.amazonaws.com", s3BucketName, awsRegion)),
				OriginAccessControlId: originAccessControl.ID(),
				S3OriginConfig: &cloudfront.DistributionOriginS3OriginConfigArgs{
					OriginAccessIdentity: pulumi.String(""),
				},
			},
		},
		OrderedCacheBehaviors: cloudfront.DistributionOrderedCacheBehaviorArray{
			&cloudfront.DistributionOrderedCacheBehaviorArgs{
				PathPattern:          pulumi.String("_images/*"),
				TargetOriginId:       pulumi.String("S3Origin"),
				ViewerProtocolPolicy: pulumi.String("allow-all"),
				AllowedMethods:       pulumi.StringArray{pulumi.String("GET"), pulumi.String("HEAD")},
				CachedMethods:        pulumi.StringArray{pulumi.String("GET"), pulumi.String("HEAD")},
				CachePolicyId:        cachePolicy.ID(),
			},
		},
		DefaultCacheBehavior: &cloudfront.DistributionDefaultCacheBehaviorArgs{
			CachePolicyId:        cachePolicy.ID(),
			TargetOriginId:       pulumi.String("S3Origin"),
			ViewerProtocolPolicy: pulumi.String("redirect-to-https"),
			AllowedMethods:       pulumi.StringArray{pulumi.String("GET"), pulumi.String("HEAD")},
			CachedMethods:        pulumi.StringArray{pulumi.String("GET"), pulumi.String("HEAD")},
		},
		Restrictions: &cloudfront.DistributionRestrictionsArgs{
			GeoRestriction: &cloudfront.DistributionRestrictionsGeoRestrictionArgs{
				RestrictionType: pulumi.String("none"),
			},
		},
		ViewerCertificate: &cloudfront.DistributionViewerCertificateArgs{
			CloudfrontDefaultCertificate: pulumi.Bool(true),
		},
	}, pulumi.Provider(awsProvider))
	if err != nil {
		return nil, fmt.Errorf("failed to create CloudFront distribution: %w", err)
	}

	// Allow this distribution (via OAC) to read meme images from S3 — matches cloudfront.yaml S3Policy.
	bucketPolicyDoc := distribution.Arn.ApplyT(func(distributionArn string) (string, error) {
		return fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Sid": "Read access for CDN",
				"Effect": "Allow",
				"Principal": {
					"Service": "cloudfront.amazonaws.com"
				},
				"Action": "s3:GetObject",
				"Resource": "arn:aws:s3:::%s/_images/*",
				"Condition": {
					"StringEquals": {
						"aws:SourceArn": "%s"
					}
				}
			}]
		}`, s3BucketName, distributionArn), nil
	}).(pulumi.StringOutput)

	bucketPolicy, err := s3.NewBucketPolicy(ctx, "meme-generator-cdn-bucket-policy", &s3.BucketPolicyArgs{
		Bucket: pulumi.String(s3BucketName),
		Policy: bucketPolicyDoc,
	}, pulumi.Provider(awsProvider), pulumi.DependsOn([]pulumi.Resource{distribution}))
	if err != nil {
		return nil, fmt.Errorf("failed to create S3 bucket policy for CloudFront: %w", err)
	}

	return &CDNResources{
		CachePolicy:         cachePolicy,
		OriginAccessControl: originAccessControl,
		Distribution:        distribution,
		BucketPolicy:        bucketPolicy,
	}, nil
}
