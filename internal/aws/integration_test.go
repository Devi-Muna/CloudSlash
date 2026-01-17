package aws

import (
	"context"
	"testing"
	"time"

	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// TestIntegrationScanVolumes runs against a LocalStack instance.
// Ensure LocalStack is running on localhost:4566
func TestIntegrationScanVolumes(t *testing.T) {
	// Skip if short mode (go test -short)
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.TODO()

	// 1. Configure SDK for LocalStack
	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     "test",
				SecretAccessKey: "test",
			}, nil
		})),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           "http://localhost:4566",
				SigningRegion: "us-east-1",
			}, nil
		})),
	)
	if err != nil {
		t.Fatalf("unable to load SDK config: %v", err)
	}

	client := ec2.NewFromConfig(cfg)

	// 2. Teardown / Cleanup
	// (Optional: Implement cleanup to ensure clean state, but LocalStack is ephemeral usually)

	// 3. Seed Data
	volOut, err := client.CreateVolume(ctx, &ec2.CreateVolumeInput{
		AvailabilityZone: aws.String("us-east-1a"),
		Size:             aws.Int32(50),
		VolumeType:       types.VolumeTypeGp2,
	})
	if err != nil {
		t.Logf("Failed to create volume (LocalStack might not be running): %v", err)
		t.Skip("LocalStack not available, skipping integration test")
		return // Explicit return needed
	}
	volID := *volOut.VolumeId
	t.Logf("Created Dummy Volume: %s", volID)

	// Wait briefly to ensure LocalStack consistency.
	time.Sleep(1 * time.Second)

	// 4. Run Scanner
	g := graph.NewGraph()
	scanner := &EC2Scanner{
		Client: client, // The real client pointing to LocalStack implements the interface
		Graph:  g,
	}

	if err := scanner.ScanVolumes(ctx); err != nil {
		t.Fatalf("ScanVolumes failed: %v", err)
	}

	// Assert
	// Verify ARN construction matches scanner implementation.
	// Current Implementation: arn:aws:ec2:region:account:volume/ID
	targetARN := "arn:aws:ec2:region:account:volume/" + volID

	node := g.GetNode(targetARN)
	if node == nil {
		// Debug dump
		for _, n := range g.Nodes {
			t.Logf("Found node: %s", n.ID)
		}
		t.Fatalf("Scanner failed to find volume %s in graph", volID)
	}

	if node.Properties["Size"] != int32(50) {
		t.Errorf("Expected size 50, got %v", node.Properties["Size"])
	}
}
