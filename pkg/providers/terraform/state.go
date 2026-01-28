package terraform

import (
	"encoding/json"
	"fmt"
)

// TerraformState represents Terraform state JSON.
//
type TerraformState struct {
	Resources []Resource `json:"resources"`
}

// Resource represents a state resource block.
//
type Resource struct {
	Module    string     `json:"module,omitempty"` // e.g. "module.payments_cluster"
	Mode      string     `json:"mode"`             // "managed" or "data"
	Type      string     `json:"type"`             // e.g. "aws_s3_bucket"
	Name      string     `json:"name"`             // e.g. "logs"
	Instances []Instance `json:"instances"`
}

// Instance represents a resource instance.
type Instance struct {
	Attributes json.RawMessage `json:"attributes"` // Resource attributes.
}

// ParsedAttribute contains common identifiers.
//
type ParsedAttribute struct {
	ID  string `json:"id"`
	ARN string `json:"arn"`
}

// ParseState parses state JSON.
func ParseState(jsonBytes []byte) (*TerraformState, error) {
	var state TerraformState
	if err := json.Unmarshal(jsonBytes, &state); err != nil {
		return nil, fmt.Errorf("failed to parse terraform state: %w", err)
	}
	return &state, nil
}
// FindAddressByID finds resource address by ID.
func FindAddressByID(state *TerraformState, cloudID string) (string, error) {
	for _, res := range state.Resources {
		if res.Mode != "managed" {
			continue
		}

		base := fmt.Sprintf("%s.%s", res.Type, res.Name)
		if res.Module != "" {
			base = fmt.Sprintf("%s.%s", res.Module, base)
		}

		for i, inst := range res.Instances {
			var attrs ParsedAttribute
			if err := json.Unmarshal(inst.Attributes, &attrs); err != nil {
				continue
			}
			if attrs.ID == cloudID || attrs.ARN == cloudID {
				if len(res.Instances) > 1 {
					return fmt.Sprintf("%s[%d]", base, i), nil
				}
				return base, nil
			}
		}
	}
	return "", nil
}
