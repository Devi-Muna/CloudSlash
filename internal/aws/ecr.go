package aws

import (
	"context"
	"errors"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
)

type ECRScanner struct {
	Client *ecr.Client
}

func NewECRScanner(cfg aws.Config) *ECRScanner {
	return &ECRScanner{
		Client: ecr.NewFromConfig(cfg),
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
}
