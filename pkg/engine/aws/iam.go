package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

type IAMClient struct {
	Client *iam.Client
}

func NewIAMClient(cfg aws.Config) *IAMClient {
	return &IAMClient{
		Client: iam.NewFromConfig(cfg),
	}
}

// CheckAdminPrivileges checks if a role has AdministratorAccess policy attached.
func (c *IAMClient) CheckAdminPrivileges(ctx context.Context, roleName string) (bool, error) {
	paginator := iam.NewListAttachedRolePoliciesPaginator(c.Client, &iam.ListAttachedRolePoliciesInput{
		RoleName: aws.String(roleName),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to list attached policies: %v", err)
		}

		for _, policy := range page.AttachedPolicies {
			if strings.HasSuffix(*policy.PolicyArn, "AdministratorAccess") {
				return true, nil
			}
		}
	}

	return false, nil
}

// GetRolesFromInstanceProfile returns the role names associated with a profile name.
func (c *IAMClient) GetRolesFromInstanceProfile(ctx context.Context, profileName string) ([]string, error) {
	out, err := c.Client.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{
		InstanceProfileName: aws.String(profileName),
	})
	if err != nil {
		return nil, err
	}

	var roles []string
	for _, role := range out.InstanceProfile.Roles {
		roles = append(roles, *role.RoleName)
	}
	return roles, nil
}
