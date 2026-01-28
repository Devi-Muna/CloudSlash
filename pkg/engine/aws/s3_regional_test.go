package aws

import (
	"context"
	"testing"

	"github.com/DrSkyle/cloudslash/pkg/graph"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// MockS3RegionalClient is a minimal mock for testing regional logic
type MockS3RegionalClient struct {
	ListBucketsFunc                 func(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
	GetBucketLocationFunc           func(ctx context.Context, params *s3.GetBucketLocationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error)
	GetBucketLifecycleConfigurationFunc func(ctx context.Context, params *s3.GetBucketLifecycleConfigurationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLifecycleConfigurationOutput, error)
	ListMultipartUploadsFunc        func(ctx context.Context, params *s3.ListMultipartUploadsInput, optFns ...func(*s3.Options)) (*s3.ListMultipartUploadsOutput, error)
}

// Ensure the S3Scanner treats our mock as a *s3.Client is hard because *s3.Client is a struct, not an interface.
// However, our code uses *s3.Client directly.
// To test this PROPERLY without integration tests, we need to inspect the 'getRegionalClient' logic directly or refactor S3Scanner to accept an interface.
// Given strict "no refactor" on interfaces request earlier and trying to stay minimal:
// We will test the `getRegionalClient` method logic via a lightweight test.

func TestGetRegionalClient_Caching(t *testing.T) {
	g := graph.NewGraph()
	cfg := aws.Config{Region: "us-east-1"}
	scanner := NewS3Scanner(cfg, g)

	// 1. Request default region (empty string)
	clientDefault := scanner.getRegionalClient("")
	if clientDefault != scanner.Client {
		t.Error("Expected default client for empty region")
	}

	// 2. Request new region "eu-west-1"
	clientEU := scanner.getRegionalClient("eu-west-1")
	if clientEU == scanner.Client {
		t.Error("Expected DIFFERENT client for eu-west-1, got base client")
	}

	// 3. Request "eu-west-1" again (Cache Hit)
	clientEUCached := scanner.getRegionalClient("eu-west-1")
	if clientEUCached != clientEU {
		t.Error("Expected SAME client instance (cache hit) for eu-west-1")
	}

	// 4. Request "us-west-2" (New Client)
	clientWest := scanner.getRegionalClient("us-west-2")
	if clientWest == clientEU || clientWest == scanner.Client {
		t.Error("Expected new unique client for us-west-2")
	}
}

// Note: Testing the full flow requires mocking the AWS SDK *Client struct which is complex.
// The caching test above proves the core mechanic: we ARE switching clients based on input strings.
