//go:build integration

package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
)

// TestFullCycle_Integration uses Testcontainers to spin up LocalStack.
// This is a "Hermetic" test: it brings its own cloud.
// Requires Docker.
func TestFullCycle_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()

	// 1. Start LocalStack Container
	container, err := localstack.RunContainer(ctx,
		testcontainers.WithImage("localstack/localstack:3.0"),
	)
	if err != nil {
		t.Fatalf("Failed to start LocalStack: %v", err)
	}
	defer func() {
		if err := container.Terminate(ctx); err != nil {
			t.Errorf("failed to terminate container: %v", err)
		}
	}()

	// 2. Configure AWS SDK to talk to LocalStack
	// Mapped port (e.g., localhost:54321 -> 4566)
	endpoint, err := container.PortEndpoint(ctx, "4566/tcp", "")
	if err != nil {
		t.Fatalf("Failed to get endpoint: %v", err)
	}

	// Custom Resolver for LocalStack
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL:           "http://" + endpoint,
			SigningRegion: "us-east-1",
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     "test",
				SecretAccessKey: "test",
				SessionToken:    "test",
			}, nil
		})),
	)
	if err != nil {
		t.Fatalf("Failed to load SDK config: %v", err)
	}

	// 3. Seed Data (Create "Waste" Infrastructure)
	ec2Client := ec2.NewFromConfig(cfg)

	runOut, err := ec2Client.RunInstances(ctx, &ec2.RunInstancesInput{
		ImageId:      aws.String("ami-12345678"),
		InstanceType: types.InstanceTypeT2Micro,
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
	})
	if err != nil {
		t.Fatalf("Failed to run instance: %v", err)
	}
	instanceID := *runOut.Instances[0].InstanceId
	t.Logf("Seeded Instance: %s", instanceID)

	// 4. Run CloudSlash Engine against this "Cloud"
	// (Here we would initialize the engine with this cfg and verify it finds the instance)
	// engine.Run(ctx, cfg, ...)

	// For now, we simulate the verification:
	descOut, err := ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		t.Fatalf("Failed to describe instances: %v", err)
	}

	if len(descOut.Reservations) == 0 {
		t.Error("LocalStack/SDK Integration failure: Instance not found")
	}
}
