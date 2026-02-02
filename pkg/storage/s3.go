package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Store implements BlobStore for AWS S3.
type S3Store struct {
	Client *s3.Client
	Bucket string
}

func NewS3Store(cfg aws.Config, bucket string) *S3Store {
	return &S3Store{
		Client: s3.NewFromConfig(cfg),
		Bucket: bucket,
	}
}

func (s *S3Store) Put(ctx context.Context, key string, data []byte) error {
	_, err := s.Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return fmt.Errorf("failed to upload to s3: %w", err)
	}
	return nil
}

func (s *S3Store) Get(ctx context.Context, key string) ([]byte, error) {
	out, err := s.Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.Bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to download from s3: %w", err)
	}
	defer out.Body.Close()

	return io.ReadAll(out.Body)
}

func (s *S3Store) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	paginator := s3.NewListObjectsV2Paginator(s.Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(s.Bucket),
		Prefix: aws.String(prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list s3 objects: %w", err)
		}
		for _, obj := range page.Contents {
			if obj.Key != nil {
				keys = append(keys, *obj.Key)
			}
		}
	}
	return keys, nil
}
