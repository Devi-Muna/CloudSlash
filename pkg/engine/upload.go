package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// UploadArtifacts uploads the contents of outputDir to the s3Target.
func (e *Engine) UploadArtifacts(ctx context.Context) error {
	if e.s3Target == "" {
		return nil
	}

	target := strings.TrimPrefix(e.s3Target, "s3://")
	parts := strings.SplitN(target, "/", 2)
	bucket := parts[0]
	prefix := ""
	if len(parts) > 1 {
		prefix = parts[1]
	}
	
	// Create minimal S3 client using standard shared config
	// We re-load config here to ensure we pick up fresh credentials if needed,
	// or we could reuse e.Pricing's config if accessible, but independent is safer for simple uploads.
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load aws config for upload: %w", err)
	}
	client := s3.NewFromConfig(cfg)

	e.Logger.Info("Uploading artifacts to S3", "bucket", bucket, "prefix", prefix)

	return filepath.Walk(e.outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(e.outputDir, path)
		if err != nil {
			return err
		}

		key := filepath.Join(prefix, relPath)
		// Ensure forward slashes for S3 keys even on Windows
		key = strings.ReplaceAll(key, "\\", "/")

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()

		_, err = client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   f,
		})
		if err != nil {
			e.Logger.Warn("Failed to upload artifact", "file", relPath, "error", err)
			// We continue uploading other files even if one fails
			return nil 
		}

		return nil
	})
}
