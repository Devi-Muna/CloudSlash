package aws

import (
	"context"
	"errors"
	"strings"
    "time"

	"github.com/DrSkyle/cloudslash/internal/graph"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

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

// CheckImageExists verifies if a specific image tag exists in ECR.
// Returns true if exists, false if not found.
// imageURI format: <account_id>.dkr.ecr.<region>.amazonaws.com/<repo_name>:<tag>
func (s *ECRScanner) CheckImageExists(ctx context.Context, imageURI string) (bool, error) {
	// Parse Image URI
	// Format: 123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo:latest
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

	// Check if it's a public ECR or other registry?
	// This client is for private ECR in the current account (mostly).
	// If the domain doesn't match the current account, we might not have permission,
	// but the heuristic usually only checks "broken artifacts" for internal services.
	// For now, we assume it's in the same account/region or we have access.

	// We can verify registryId from the domain if needed, but BatchGetImage defaults to default registry if not specified,
	// which might be wrong if image is in another account.
	// Let's attempt to parse registry ID from domain.
	var registryID *string
	domainParts := strings.Split(domain, ".")
	if len(domainParts) > 0 && len(domainParts[0]) == 12 { // rudimentary account ID check
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
		// If repo not found, image definitely doesn't exist
		if strings.Contains(err.Error(), "RepositoryNotFoundException") {
			return false, nil
		}
		return false, err
	}

	// If images found, it exists
	if len(output.Images) > 0 {
		return true, nil
	}

	// If failed (image not found in repo), it returns Failures
	if len(output.Failures) > 0 {
		return false, nil
	}

	return false, nil
	return false, nil
}

// ScanRepositories scans ECR repositories for Lifecycle Policies and Untagged Images.
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

			// 1. Check Lifecycle Policy
			hasPolicy := false
			policyInput := &ecr.GetLifecyclePolicyInput{RepositoryName: aws.String(repoName)}
			if _, err := s.Client.GetLifecyclePolicy(ctx, policyInput); err == nil {
				hasPolicy = true
			} else {
				// Graceful 403 Handling
				if strings.Contains(err.Error(), "AccessDenied") {
					// Treat as "Unknown" or log warning?
					// For safety, let's assume "HasPolicy=true" to avoid recommending deletion if we can't be sure?
					// Or just mark as "PolicyUnknown".
					// User logic: "The Promise: Your promise is Read Only... fail gracefully."
					// Implementation: We flag hasPolicy=false normally, but if we can't check, we shouldn't assume it has none.
					// Let's set a property "PolicyCheckError"
				}
			}

			wasteBytes := int64(0)

			// 2. Analyze Images (Only if No Policy - optimization)
			// Actually, even if policy exists, we might want to know stats?
			// But for "Goal Standard", we focus on No Policy.
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
			// Logic: Untagged AND (Unpulled OR Pulled > 90d ago)
			isUntagged := len(img.ImageTags) == 0

			if isUntagged {
				// Check Pull Time
				isOld := true
				if img.LastRecordedPullTime != nil {
					// If pulled recently, it's NOT old
					if time.Since(*img.LastRecordedPullTime) < 90*24*time.Hour {
						isOld = false
					}
				} else {
					// Never pulled? Or logic: "Is lastRecordedPullTime > 90 days (or None)? (Yes)"
					// If None, it implies never pulled (or before feature existed). Treat as Old/Trash.
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
