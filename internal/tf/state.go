package tf

import (
	"encoding/json"
	"fmt"
	"os"
)

// State represents Terraform state file.
type State struct {
	Version          int        `json:"version"`
	TerraformVersion string     `json:"terraform_version"`
	Resources        []Resource `json:"resources"`
}

// Resource represents a state resource.
type Resource struct {
	Mode      string     `json:"mode"`
	Type      string     `json:"type"`
	Name      string     `json:"name"`
	Provider  string     `json:"provider"`
	Instances []Instance `json:"instances"`
}

// Instance represents a resource instance.
type Instance struct {
	Attributes map[string]interface{} `json:"attributes"`
}

// ParseStateFile reads state file.
func ParseStateFile(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %v", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state JSON: %v", err)
	}

	return &state, nil
}

// GetManagedResourceIDs returns managed IDs.
// 
func (s *State) GetManagedResourceIDs() map[string]bool {
	managed := make(map[string]bool)

	for _, res := range s.Resources {
		for _, inst := range res.Instances {
			// Index ID and ARN.
			if id, ok := inst.Attributes["id"].(string); ok {
				managed[id] = true
			}
			if arn, ok := inst.Attributes["arn"].(string); ok {
				managed[arn] = true
			}
		}
	}

	return managed
}

// GetResourceMapping maps IDs to addresses.
//
func (s *State) GetResourceMapping() map[string]string {
	mapping := make(map[string]string)

	for _, res := range s.Resources {
		// Build address.
		address := fmt.Sprintf("%s.%s", res.Type, res.Name)

		for _, inst := range res.Instances {
			if id, ok := inst.Attributes["id"].(string); ok {
				mapping[id] = address
			}
			if arn, ok := inst.Attributes["arn"].(string); ok {
				mapping[arn] = address
			}
		}
	}
	return mapping
}
