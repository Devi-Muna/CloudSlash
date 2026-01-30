package aws

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Client encapsulates AWS SDK usage, handling authentication, region resolution, and middleware injection.
type Client struct {
	Config aws.Config
	STS    *sts.Client
}

// NewClient initializes a new authenticated AWS client.
func NewClient(ctx context.Context, region, profile string, verbose bool) (*Client, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	// Check for local endpoint overrides (used for mocking/testing).
	if endpoint := os.Getenv("AWS_ENDPOINT_URL"); endpoint != "" {
		opts = append(opts, config.WithBaseEndpoint(endpoint))
	}

	// Define the application signature for backend audit logging.
	const signature = "CS-v1-7f8a9d-AGPL"

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %v", err)
	}

	// Inject custom User-Agent header for usage tracking and API quotas.
	cfg.APIOptions = append(cfg.APIOptions, func(stack *middleware.Stack) error {
		return stack.Build.Add(middleware.BuildMiddlewareFunc("UserAgentTrap", func(ctx context.Context, input middleware.BuildInput, next middleware.BuildHandler) (
			middleware.BuildOutput, middleware.Metadata, error,
		) {
			req, ok := input.Request.(*smithyhttp.Request)
			if ok {
				currentUA := req.Header.Get("User-Agent")
				if currentUA == "" {
					currentUA = "CloudSlash/v1.1"
				}
				req.Header.Set("User-Agent", fmt.Sprintf("%s (%s)", currentUA, signature))
			}
			return next.HandleBuild(ctx, input)
		}), middleware.After)
	})

	// If matrix mode is enabled, intercept and log all API calls for visual debugging.
	if verbose {
		cfg.APIOptions = append(cfg.APIOptions, func(stack *middleware.Stack) error {
			return stack.Initialize.Add(middleware.InitializeMiddlewareFunc("MatrixLogger", func(ctx context.Context, input middleware.InitializeInput, next middleware.InitializeHandler) (
				middleware.InitializeOutput, middleware.Metadata, error,
			) {
				opName := middleware.GetOperationName(ctx)
				fmt.Printf("\033[2m\033[32m[AWS-SDK] API Call: %s\033[0m\n", opName)
				return next.HandleInitialize(ctx, input)
			}), middleware.Before)
		})
	}

	return &Client{
		Config: cfg,
		STS:    sts.NewFromConfig(cfg),
	}, nil
}

// VerifyIdentity validates the session credentials and retrieves the canonical Account ID.
func (c *Client) VerifyIdentity(ctx context.Context) (string, error) {
	input := &sts.GetCallerIdentityInput{}
	result, err := c.STS.GetCallerIdentity(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to get caller identity: %v", err)
	}
	return *result.Account, nil
}

// GetConfigForRegion returns a regional configuration copy.
// Use this when interacting with cross-region resources (like S3 buckets detailed in a different region).
func (c *Client) GetConfigForRegion(region string) aws.Config {
	cfg := c.Config.Copy()
	cfg.Region = region
	return cfg
}

// ListProfiles attempts to resolve all configured AWS profiles on the host system.
func ListProfiles() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	profiles := make(map[string]bool)
	paths := []string{}
	// Check standard environment variables.
	if cfgPath := os.Getenv("AWS_CONFIG_FILE"); cfgPath != "" {
		paths = append(paths, cfgPath)
	} else {
		paths = append(paths, filepath.Join(home, ".aws", "config"))
	}

	if credPath := os.Getenv("AWS_SHARED_CREDENTIALS_FILE"); credPath != "" {
		paths = append(paths, credPath)
	} else {
		paths = append(paths, filepath.Join(home, ".aws", "credentials"))
	}

	// Parse profile names using regex.
	re := regexp.MustCompile(`^\[(?:profile\s+)?([^\]]+)\]`)

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			continue // Skip if file doesn't exist
		}

		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				profiles[matches[1]] = true
			}
		}
	}

	var list []string
	for p := range profiles {
		list = append(list, p)
	}

	if len(list) == 0 {
		// Fallback: Check for Web Identity (IRSA/EKS) presence.
		if os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE") != "" {
			return []string{"default"}, nil
		}
		// Fallback: Check for standard environment credentials.
		if os.Getenv("AWS_ACCESS_KEY_ID") != "" {
			return []string{"default"}, nil
		}

		return nil, fmt.Errorf("no profiles found in standard locations")
	}

	return list, nil
}
