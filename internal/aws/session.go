package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// Client holds AWS service clients.
type Client struct {
	Config aws.Config
	STS    *sts.Client
}

// NewClient initializes a new AWS client with default config.
func NewClient(ctx context.Context, region string) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %v", err)
	}

	return &Client{
		Config: cfg,
		STS:    sts.NewFromConfig(cfg),
	}, nil
}

// VerifyIdentity checks if the credentials are valid and returns the caller identity.
func (c *Client) VerifyIdentity(ctx context.Context) (string, error) {
	input := &sts.GetCallerIdentityInput{}
	result, err := c.STS.GetCallerIdentity(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to get caller identity: %v", err)
	}
	return *result.Account, nil
}
