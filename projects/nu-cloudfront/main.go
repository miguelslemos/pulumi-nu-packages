package main

import (
	"encoding/json"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudfront"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/s3"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

type DistributionRestrictionsGeoRestriction struct {
	Locations       []string `pulumi:"locations"`
	RestrictionType string   `pulumi:"restrictionType"`
}

type DistributionRestrictions struct {
	GeoRestriction DistributionRestrictionsGeoRestriction `pulumi:"geoRestriction"`
}

type DistributionViewerCertificateArgs struct {
	AcmCertificateArn            *string
	CloudfrontDefaultCertificate *bool
	IamCertificateId             *string
	MinimumProtocolVersion       *string
	SslSupportMethod             *string
}

type DistributionOrigin struct {
	ConnectionAttempts    *int `pulumi:"connectionAttempts"`
	ConnectionTimeout     *int `pulumi:"connectionTimeout"`
	CustomHeaders         cloudfront.DistributionOriginCustomHeaderArray
	CustomOriginConfig    cloudfront.DistributionOriginCustomOriginConfigPtrInput `pulumi:"customOriginConfig"`
	DomainName            string                                                  `pulumi:"domainName"`
	OriginAccessControlId *string                                                 `pulumi:"originAccessControlId"`
	OriginId              string                                                  `pulumi:"originId"`
	OriginPath            *string                                                 `pulumi:"originPath"`
	OriginShield          cloudfront.DistributionOriginOriginShieldPtrInput       `pulumi:"originShield"`
	S3OriginConfig        cloudfront.DistributionOriginS3OriginConfigPtrInput     `pulumi:"s3OriginConfig"`
}

type distributionArgs struct {
	Aliases                      []string
	Comment                      string
	ContinuousDeploymentPolicyId string
	CustomErrorResponses         cloudfront.DistributionCustomErrorResponseArray
	DefaultCacheBehavior         cloudfront.DistributionDefaultCacheBehavior      `pulumi:"defaultCacheBehavior"`
	DefaultRootObject            string                                           `pulumi:"defaultRootObject"`
	Enabled                      bool                                             `pulumi:"enabled"`
	HttpVersion                  string                                           `pulumi:"httpVersion"`
	IsIpv6Enabled                bool                                             `pulumi:"isIpv6Enabled"`
	LoggingConfig                cloudfront.DistributionLoggingConfigPtrInput     `pulumi:"loggingConfig"`
	OrderedCacheBehaviors        cloudfront.DistributionOrderedCacheBehaviorArray `pulumi:"orderedCacheBehaviors"`
	OriginGroups                 cloudfront.DistributionOriginGroupArray          `pulumi:"originGroups"`
	// Origins                      []DistributionOrigin
	PriceClass        string `pulumi:"priceClass"`
	Restrictions      DistributionRestrictions
	Staging           bool              `pulumi:"staging"`
	Tags              map[string]string `pulumi:"tags"`
	ViewerCertificate DistributionViewerCertificateArgs
	WaitForDeployment bool   `pulumi:"waitForDeployment"`
	WebAclId          string `pulumi:"webAclId"`
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		config := config.New(ctx, "")
		bucketName := config.Require("bucketName")
		var distributionArgs distributionArgs
		config.RequireObject("distribution", &distributionArgs)

		// Generate Origin Access Identity to access the private s3 bucket.
		originAccessIdentity, err := cloudfront.NewOriginAccessIdentity(ctx, "originAccessIdentity", &cloudfront.OriginAccessIdentityArgs{
			Comment: pulumi.String("this is needed to setup s3 polices and make s3 not public."),
		})
		if err != nil {
			return err
		}

		bucketContent, err := s3.LookupBucket(ctx, &s3.LookupBucketArgs{
			Bucket: bucketName,
		}, nil)

		if err != nil {
			return err
		}

		// Create a bucket policy to allow cloudfront to access the bucket.
		_, err = s3.NewBucketPolicy(ctx, "cloudfront-bucket-policy", &s3.BucketPolicyArgs{
			Bucket: pulumi.String(bucketContent.Id),
			Policy: pulumi.All(bucketContent.Arn, originAccessIdentity.IamArn).ApplyT(
				func(args []any) (string, error) {
					bucketArn := args[0].(string)
					iamArn := args[1].(string)
					policy, err := json.Marshal(map[string]any{
						"Version": "2012-10-17",
						"Statement": []map[string]any{
							{
								"Sid":    "CloudfrontAllow",
								"Effect": "Allow",
								"Principal": map[string]any{
									"AWS": iamArn,
								},
								"Action":   "s3:GetObject",
								"Resource": bucketArn + "/*",
							},
						},
					})
					if err != nil {
						return "", err
					}
					return string(policy), nil
				}).(pulumi.StringOutput),
		})

		if err != nil {
			return err
		}

		// Create a cloudfront distribution to serve the content from the bucket.
		distribution, err := cloudfront.NewDistribution(ctx, "distribution", &cloudfront.DistributionArgs{
			Aliases: pulumi.ToStringArray(distributionArgs.Aliases),
			Comment: pulumi.StringPtr(distributionArgs.Comment),
			// Origins: distributionArgs.Origins,
			// DefaultCacheBehavior:         distributionArgs.DefaultCacheBehavior,
			ContinuousDeploymentPolicyId: pulumi.StringPtr(distributionArgs.ContinuousDeploymentPolicyId),
			CustomErrorResponses:         distributionArgs.CustomErrorResponses,
			DefaultRootObject:            pulumi.StringPtr(distributionArgs.DefaultRootObject),
			Enabled:                      pulumi.Bool(distributionArgs.Enabled),
			HttpVersion:                  pulumi.StringPtr(distributionArgs.HttpVersion),
			IsIpv6Enabled:                pulumi.BoolPtr(distributionArgs.IsIpv6Enabled),
			LoggingConfig:                distributionArgs.LoggingConfig,
			OrderedCacheBehaviors:        distributionArgs.OrderedCacheBehaviors,
			PriceClass:                   pulumi.StringPtr(distributionArgs.PriceClass),
			Restrictions: cloudfront.DistributionRestrictionsArgs{
				GeoRestriction: cloudfront.DistributionRestrictionsGeoRestrictionArgs{
					Locations:       pulumi.ToStringArray(distributionArgs.Restrictions.GeoRestriction.Locations),
					RestrictionType: pulumi.String(distributionArgs.Restrictions.GeoRestriction.RestrictionType),
				},
			},
			Staging: pulumi.BoolPtr(distributionArgs.Staging),
			Tags:    pulumi.ToStringMap(distributionArgs.Tags),
			ViewerCertificate: cloudfront.DistributionViewerCertificateArgs{
				AcmCertificateArn:      pulumi.StringPtr(*distributionArgs.ViewerCertificate.AcmCertificateArn),
				MinimumProtocolVersion: pulumi.StringPtr(*distributionArgs.ViewerCertificate.MinimumProtocolVersion),
				SslSupportMethod:       pulumi.StringPtr(*distributionArgs.ViewerCertificate.SslSupportMethod),
			},
			WaitForDeployment: pulumi.BoolPtr(distributionArgs.WaitForDeployment),
			WebAclId:          pulumi.StringPtr(distributionArgs.WebAclId),
		})
		if err != nil {
			return err
		}

		ctx.Export("contentBucketUri", pulumi.String(bucketContent.Bucket))
		ctx.Export("contentBucketWebsiteEndpoint", pulumi.String(bucketContent.WebsiteEndpoint))
		ctx.Export("cloudFrontDomain", distribution.DomainName)
		return nil
	})
}