package aws

import (
	"context"
	"fmt"
	
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
)

type IAMClient struct {
	Client *iam.Client
}

func NewIAMClient(cfg aws.Config) *IAMClient {
	return &IAMClient{
		Client: iam.NewFromConfig(cfg),
	}
}

// SimulatePrivileges verifies permissions using the AWS Policy Simulator.
// Returns potentially dangerous permissions granted globally ("*").
func (c *IAMClient) SimulatePrivileges(ctx context.Context, roleArn string) ([]string, error) {
	// Verify if the role can perform destructive actions globally.
	// Checks for wildcard permission grants.
	dangerousActions := []string{
		"s3:DeleteBucket",
		"ec2:TerminateInstances",
		"rds:DeleteDBInstance",
		"iam:CreateUser",
		"iam:AttachRolePolicy",
	}

	input := &iam.SimulatePrincipalPolicyInput{
		PolicySourceArn: aws.String(roleArn),
		ActionNames:     dangerousActions,
		ResourceArns:    []string{"*"}, // Check global impact
	}

	out, err := c.Client.SimulatePrincipalPolicy(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("policy simulation failed: %v", err)
	}

	var confirmedRisks []string
	for _, result := range out.EvaluationResults {
		if result.EvalDecision == types.PolicyEvaluationDecisionTypeAllowed {
			if result.EvalActionName != nil {
				confirmedRisks = append(confirmedRisks, *result.EvalActionName)
			}
		}
	}

	return confirmedRisks, nil
}

// GetRolesFromInstanceProfile retrieves role ARNs for a given profile.
func (c *IAMClient) GetRolesFromInstanceProfile(ctx context.Context, profileName string) ([]string, error) {
	out, err := c.Client.GetInstanceProfile(ctx, &iam.GetInstanceProfileInput{
		InstanceProfileName: aws.String(profileName),
	})
	if err != nil {
		return nil, err
	}

	var roleArns []string
	for _, role := range out.InstanceProfile.Roles {
		if role.Arn != nil {
			roleArns = append(roleArns, *role.Arn)
		}
	}
	return roleArns, nil
}
