package history

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Backend implements S3 storage.
type S3Backend struct {
	Bucket string
	Key    string
	Client *s3.Client
}

// NewS3Backend creates a new S3 backend.
func NewS3Backend(s3URL string) (*S3Backend, error) {
	u, err := url.Parse(s3URL)
	if err != nil {
		return nil, fmt.Errorf("invalid s3 url: %v", err)
	}

	bucket := u.Host
	key := strings.TrimPrefix(u.Path, "/")

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %v", err)
	}

	return &S3Backend{
		Bucket: bucket,
		Key:    key,
		Client: s3.NewFromConfig(cfg),
	}, nil
}

func (b *S3Backend) Append(s Snapshot) error {
	// Load existing snapshot data.
	// S3 uses read-modify-write as it lacks native append support.
	
	existing, err := b.readAll()
	if err != nil {
		// If 404, start new
		existing = []Snapshot{}
	}

	existing = append(existing, s)


	
	// Write back
	var buf bytes.Buffer
	for _, snap := range existing {
		data, _ := json.Marshal(snap)
		buf.Write(data)
		buf.WriteString("\n")
	}

	_, err = b.Client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket: aws.String(b.Bucket),
		Key:    aws.String(b.Key),
		Body:   bytes.NewReader(buf.Bytes()),
	})
	
	return err
}

func (b *S3Backend) Load(n int) ([]Snapshot, error) {
	history, err := b.readAll()
	if err != nil {
		return nil, err
	}

	if len(history) > n {
		return history[len(history)-n:], nil
	}
	return history, nil
}

func (b *S3Backend) readAll() ([]Snapshot, error) {
	resp, err := b.Client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: aws.String(b.Bucket),
		Key:    aws.String(b.Key),
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var history []Snapshot
	// Read full body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	scanner := bufio.NewScanner(bytes.NewReader(bodyBytes))
	for scanner.Scan() {
		var s Snapshot
		if err := json.Unmarshal(scanner.Bytes(), &s); err != nil {
			continue
		}
		history = append(history, s)
	}
	return history, nil
}
