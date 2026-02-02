package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/iam/types"
)

// IAMClient handles policy simulation.
type IAMClient struct {
	Client *iam.Client
}

func NewIAMClient(cfg aws.Config) *IAMClient {
	return &IAMClient{
		Client: iam.NewFromConfig(cfg),
	}
}

// SimulatePrivileges checks for dangerous actions.
func (c *IAMClient) SimulatePrivileges(ctx context.Context, roleArn string) ([]string, error) {
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
		ResourceArns:    []string{"*"}, // Check for broad, un-scoped permissions.
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

// GetRolesFromInstanceProfile retrieves associated roles.
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
