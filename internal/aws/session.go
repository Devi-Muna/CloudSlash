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

// Client holds AWS service clients.
type Client struct {
	Config aws.Config
	STS    *sts.Client
}

// NewClient initializes a new AWS client.
func NewClient(ctx context.Context, region, profile string) (*Client, error) {
	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	// TRAP: User-Agent Fingerprint
	// If a competitor forks this codebase but forgets to change this,
	// their API calls will be permanently stamped with "CS-v1-7f8a9d-AGPL".
	const signature = "CS-v1-7f8a9d-AGPL"

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %v", err)
	}
	
	// Inject signature
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

// ListProfiles attempts to find all profiles in ~/.aws/config and ~/.aws/credentials.
func ListProfiles() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	profiles := make(map[string]bool)
	paths := []string{}
	// Check standard env vars first
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

	// Regex to find [profile name] or [name]
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
		return nil, fmt.Errorf("no profiles found in standard locations")
	}

	return list, nil
}
