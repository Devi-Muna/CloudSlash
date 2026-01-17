package aws

import (
	"context"
	"fmt"

	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

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

// getRegionalClient ensures we talk to the region where the bucket actually lives.
func (s *S3Scanner) getRegionalClient(region string) *s3.Client {
	if region == "" {
		return s.Client
	}
	if client, ok := s.RegionalClients[region]; ok {
		return client
	}
	// Create new client for this region
	cfg := s.BaseConfig.Copy()
	cfg.Region = region
	client := s3.NewFromConfig(cfg)
	s.RegionalClients[region] = client
	return client
}



func (s *S3Scanner) ScanBuckets(ctx context.Context) error {
	result, err := s.Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return fmt.Errorf("failed to list buckets: %v", err)
	}

	for _, bucket := range result.Buckets {
		name := *bucket.Name
		arn := fmt.Sprintf("arn:aws:s3:::bucket/%s", name)

		// 1. Resolve Region to avoid 301 Redirect errors
		region := "us-east-1" // Default
		loc, err := s.Client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: &name})
		if err == nil && loc.LocationConstraint != "" {
			region = string(loc.LocationConstraint)
			// Handle legacy "EU" constraint mapping to eu-west-1
			if region == "EU" {
				region = "eu-west-1"
			}
		}
		
		// 2. Get Region-Specific Client
		regionalClient := s.getRegionalClient(region)

		props := map[string]interface{}{
			"Name":         name,
			"Region":       region,
			"CreationDate": bucket.CreationDate,
		}

		// Check for lifecycle rules using REGIONAL client
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
			s.Graph.AddEdge(arn, bucketARN) // Link to bucket
		}
	}
	return nil
}
