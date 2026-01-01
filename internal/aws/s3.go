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
	Client *s3.Client
	Graph  *graph.Graph
}

func NewS3Scanner(cfg aws.Config, g *graph.Graph) *S3Scanner {
	return &S3Scanner{
		Client: s3.NewFromConfig(cfg),
		Graph:  g,
	}
}



func (s *S3Scanner) ScanBuckets(ctx context.Context) error {
	result, err := s.Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return fmt.Errorf("failed to list buckets: %v", err)
	}

	for _, bucket := range result.Buckets {
		name := *bucket.Name
		arn := fmt.Sprintf("arn:aws:s3:::bucket/%s", name)

		props := map[string]interface{}{
			"Name":         name,
			"CreationDate": bucket.CreationDate,
		}

		// Check Lifecycle (Optimization)
		hasAbortRule := s.hasAbortLifecycle(ctx, name)
		props["HasAbortLifecycle"] = hasAbortRule
		
		s.Graph.AddNode(arn, "AWS::S3::Bucket", props)

		// Check for Multipart Uploads ONLY if strict cleaning isn't automated
		if !hasAbortRule {
			if err := s.scanMultipartUploads(ctx, name, arn); err != nil {
				fmt.Printf("Failed to scan multipart uploads for bucket %s: %v\n", name, err)
			}
		}
	}
	return nil
}

func (s *S3Scanner) hasAbortLifecycle(ctx context.Context, bucket string) bool {
	lc, err := s.Client.GetBucketLifecycleConfiguration(ctx, &s3.GetBucketLifecycleConfigurationInput{
		Bucket: aws.String(bucket),
	})
	if err != nil {
		return false // No config or error -> Assume unsafe
	}
	
	for _, rule := range lc.Rules {
		if rule.Status == types.ExpirationStatusEnabled && rule.AbortIncompleteMultipartUpload != nil {
			return true
		}
	}
	return false
}

func (s *S3Scanner) scanMultipartUploads(ctx context.Context, bucketName, bucketARN string) error {
	paginator := s3.NewListMultipartUploadsPaginator(s.Client, &s3.ListMultipartUploadsInput{
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
