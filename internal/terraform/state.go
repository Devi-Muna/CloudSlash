package terraform

import (
	"encoding/json"
	"fmt"
)

// TerraformState represents the top-level structure of a .tfstate JSON export.
// We only care about the resources for now.
type TerraformState struct {
	Resources []Resource `json:"resources"`
}

// Resource represents a single Terraform resource definition (block).
// It can contain multiple instances (e.g. count/for_each).
type Resource struct {
	Module    string     `json:"module,omitempty"` // e.g. "module.payments_cluster"
	Mode      string     `json:"mode"`             // "managed" or "data"
	Type      string     `json:"type"`             // e.g. "aws_s3_bucket"
	Name      string     `json:"name"`             // e.g. "logs"
	Instances []Instance `json:"instances"`
}

// Instance represents a realized instance of a resource.
type Instance struct {
	Attributes json.RawMessage `json:"attributes"` // Dynamic bag of attributes
}

// ParsedAttribute helps us extract common identifiers like ID or ARN
// from the raw attributes bag.
type ParsedAttribute struct {
	ID  string `json:"id"`
	ARN string `json:"arn"`
}

// ParseState parses the raw JSON output from 'terraform state pull'.
func ParseState(jsonBytes []byte) (*TerraformState, error) {
	var state TerraformState
	if err := json.Unmarshal(jsonBytes, &state); err != nil {
		return nil, fmt.Errorf("failed to parse terraform state: %w", err)
	}
	return &state, nil
}
