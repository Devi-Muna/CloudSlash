package aws

import (
	"context"
	"fmt"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Scanner scans S3 buckets and contents.
type S3Scanner struct {
	Client          *s3.Client
	BaseConfig      aws.Config
	RegionalClients map[string]*s3.Client
	Graph           *graph.Graph
}

func NewS3Scanner(cfg aws.Config, g *graph.Graph) *S3Scanner {
	return &S3Scanner{
		Client:          s3.NewFromConfig(cfg),
		BaseConfig:      cfg,
		RegionalClients: make(map[string]*s3.Client),
		Graph:           g,
	}
}

// getRegionalClient returns a region-specific S3 client.
func (s *S3Scanner) getRegionalClient(region string) *s3.Client {
	if region == "" {
		return s.Client
	}
	if client, ok := s.RegionalClients[region]; ok {
		return client
	}
	// Initialize a new client for the target region.
	cfg := s.BaseConfig.Copy()
	cfg.Region = region
	client := s3.NewFromConfig(cfg)
	s.RegionalClients[region] = client
	return client
}

// ScanBuckets analyzes S3 buckets and their configurations.
func (s *S3Scanner) ScanBuckets(ctx context.Context) error {
	result, err := s.Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return fmt.Errorf("failed to list buckets: %v", err)
	}

	for _, bucket := range result.Buckets {
		name := *bucket.Name
		arn := fmt.Sprintf("arn:aws:s3:::bucket/%s", name)

		// Find bucket region.
		var region string
		loc, err := s.Client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: &name})
		if err == nil && loc.LocationConstraint != "" {
			region = string(loc.LocationConstraint)
			// Map legacy 'EU' location constraint to 'eu-west-1'.
			if region == "EU" {
				region = "eu-west-1"
			}
		} else {
			// Without LocationConstraint, it "should" be us-east-1, but we must verify.
			// Permissions issues can also cause empty responses.
			if s.verifyBucketRegion(ctx, name, "us-east-1") {
				region = "us-east-1"
			} else {
				// Fallback: If region determination fails, mark as unknown to prevent cascading errors.
				region = "RegionUnknown"
				fmt.Printf("Warning: Could not determine region for bucket %s. Scanning metadata only.\n", name)
			}
		}

		// Get client.
		regionalClient := s.getRegionalClient(region)

		props := map[string]interface{}{
			"Name":         name,
			"Region":       region,
			"CreationDate": bucket.CreationDate,
		}

		// Check for lifecycle rules that abort incomplete multipart uploads.
		// Note: We use the regional client for GetBucketLifecycleConfiguration to avoid redirection errors.
		hasAbortRule := s.hasAbortLifecycle(ctx, regionalClient, name)
		props["HasAbortLifecycle"] = hasAbortRule

		s.Graph.AddNode(arn, "AWS::S3::Bucket", props)

		// Scan for incomplete multipart uploads if no abort rule exists.
		if !hasAbortRule {
			if err := s.scanMultipartUploads(ctx, regionalClient, name, arn); err != nil {
				fmt.Printf("Failed to scan multipart uploads for bucket %s (%s): %v\n", name, region, err)
			}
		}
	}
	return nil
}

// hasAbortLifecycle checks for multipart upload abort rules.
func (s *S3Scanner) hasAbortLifecycle(ctx context.Context, client *s3.Client, bucket string) bool {
	lc, err := client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return false // Assume unsafe configuration on error.
	}

	for _, rule := range lc.Rules {
		if rule.Status == types.ExpirationStatusEnabled && rule.AbortIncompleteMultipartUpload != nil {
			return true
		}
	}
	return false
}

// scanMultipartUploads finds incomplete multipart uploads.
func (s *S3Scanner) scanMultipartUploads(ctx context.Context, client *s3.Client, bucketName, bucketARN string) error {
	paginator := s3.NewListMultipartUploadsPaginator(client, &s3.ListMultipartUploadsInput{
		Bucket: aws.String(bucketName),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}

		for _, upload := range page.Uploads {
			key := *upload.Key
			uploadId := *upload.UploadId
			arn := fmt.Sprintf("arn:aws:s3:::multipart/%s/%s", bucketName, uploadId)

			props := map[string]interface{}{
				"Bucket":    bucketName,
				"Key":       key,
				"UploadId":  uploadId,
				"Initiated": upload.Initiated,
			}

			s.Graph.AddNode(arn, "AWS::S3::MultipartUpload", props)
			s.Graph.AddEdge(arn, bucketARN) // Establish dependency.
		}
	}
	return nil
}

// verifyBucketRegion confirms bucket accessibility in region.
func (s *S3Scanner) verifyBucketRegion(ctx context.Context, bucket, region string) bool {
	client := s.getRegionalClient(region)
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	return err == nil
}
