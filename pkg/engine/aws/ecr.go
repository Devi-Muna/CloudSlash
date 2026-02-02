package aws

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/DrSkyle/cloudslash/v2/pkg/graph"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

// ECRScanner scans Container Repositories.
type ECRScanner struct {
	Client *ecr.Client
	Graph  *graph.Graph
}

func NewECRScanner(cfg aws.Config, g *graph.Graph) *ECRScanner {
	return &ECRScanner{
		Client: ecr.NewFromConfig(cfg),
		Graph:  g,
	}
}

// CheckImageExists checks image existence.
func (s *ECRScanner) CheckImageExists(ctx context.Context, imageURI string) (bool, error) {
	// Parse URI components.
	parts := strings.Split(imageURI, "/")
	if len(parts) < 2 {
		return false, errors.New("invalid image URI format")
	}

	repoAndTag := parts[len(parts)-1]
	repoParts := strings.Split(repoAndTag, ":")
	if len(repoParts) != 2 {
		return false, errors.New("image URI must contain a tag")
	}
	repoName := repoParts[0]
	imageTag := repoParts[1]

	// To support repos in folders (e.g. my-org/my-repo), we need to rejoin parts excluding the domain
	// But ECR repo names can contain slashes.
	// The domain is always the first part.
	// e.g. domain/repo/subrepo:tag

	domain := parts[0]
	path := strings.Join(parts[1:], "/") // repo/subrepo:tag
	pathParts := strings.Split(path, ":")
	repoName = pathParts[0]
	imageTag = pathParts[1]

	// Verify registry ID to ensure we are querying the correct account.

	// We can verify registryId from the domain if needed, but BatchGetImage defaults to default registry if not specified,
	// which might be wrong if image is in another account.
	// Let's attempt to parse registry ID from domain.
	var registryID *string
	domainParts := strings.Split(domain, ".")
	if len(domainParts) > 0 && len(domainParts[0]) == 12 { // Extract account ID from domain
		registryID = aws.String(domainParts[0])
	}

	input := &ecr.BatchGetImageInput{
		RepositoryName: aws.String(repoName),
		ImageIds: []types.ImageIdentifier{
			{
				ImageTag: aws.String(imageTag),
			},
		},
	}
	if registryID != nil {
		input.RegistryId = registryID
	}

	output, err := s.Client.BatchGetImage(ctx, input)
	if err != nil {
		// Repo not found implies image not found.
		if strings.Contains(err.Error(), "RepositoryNotFoundException") {
			return false, nil
		}
		return false, err
	}

	// Images found implies existence.
	if len(output.Images) > 0 {
		return true, nil
	}

	// If failed (image not found in repo), it returns Failures
	if len(output.Failures) > 0 {
		return false, nil
	}

	return false, nil
}

// ScanRepositories scans repositories for waste.
func (s *ECRScanner) ScanRepositories(ctx context.Context) error {
	paginator := ecr.NewDescribeRepositoriesPaginator(s.Client, &ecr.DescribeRepositoriesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}

		for _, repo := range page.Repositories {
			repoName := *repo.RepositoryName
			repoArn := *repo.RepositoryArn

			// Check for existing lifecycle policies.
			hasPolicy := false
			policyInput := &ecr.GetLifecyclePolicyInput{RepositoryName: aws.String(repoName)}
			if _, err := s.Client.GetLifecyclePolicy(ctx, policyInput); err == nil {
				hasPolicy = true
			} else {
				// Handle access denied.
				if strings.Contains(err.Error(), "AccessDenied") {
					// Log warning on access error.
					// We cannot determine policy status, so we proceed cautiously.
				}
			}

			wasteBytes := int64(0)

			// If no policy exists, analyze images for staleness.
			if !hasPolicy {
				wasteBytes = s.analyzeImages(ctx, repoName)
			}

			props := map[string]interface{}{
				"Name":       repoName,
				"HasPolicy":  hasPolicy,
				"WasteBytes": wasteBytes,
			}

			s.Graph.AddNode(repoArn, "AWS::ECR::Repository", props)
		}
	}
	return nil
}

func (s *ECRScanner) analyzeImages(ctx context.Context, repoName string) int64 {
	var wasteBytes int64
	paginator := ecr.NewDescribeImagesPaginator(s.Client, &ecr.DescribeImagesInput{
		RepositoryName: aws.String(repoName),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			break // Skip if error
		}

		for _, img := range page.ImageDetails {
			// Identify waste.
			isUntagged := len(img.ImageTags) == 0

			if isUntagged {
				// Check pull time.
				isOld := true
				if img.LastRecordedPullTime != nil {
					// Recent pull.
					if time.Since(*img.LastRecordedPullTime) < 90*24*time.Hour {
						isOld = false
					}
				} else {
					// Stale.
				}

				if isOld {
					if img.ImageSizeInBytes != nil {
						wasteBytes += *img.ImageSizeInBytes
					}
				}
			}
		}
	}
	return wasteBytes
}
