package permissions

import (
	"encoding/json"
	"sort"
)

type PolicyDocument struct {
	Version   string      `json:"Version"`
	Statement []Statement `json:"Statement"`
}

type Statement struct {
	Sid      string   `json:"Sid"`
	Effect   string   `json:"Effect"`
	Action   []string `json:"Action"`
	Resource string   `json:"Resource"`
}

// GeneratePolicy creates a least-privilege IAM policy based on enabled modules.
// If modules is empty, it returns the full policy for all supported scanners.
func GeneratePolicy(modules []string) ([]byte, error) {
	desiredActions := make(map[string]bool)

	// Add Core Permissions
	for _, perm := range CorePermissions() {
		desiredActions[perm] = true
	}

	// Add Module Permissions
	if len(modules) == 0 {
		// Enable All
		for _, perms := range Catalog {
			for _, p := range perms {
				desiredActions[p] = true
			}
		}
	} else {
		// Enable Selected
		for _, mod := range modules {
			if perms, ok := Catalog[mod]; ok {
				for _, p := range perms {
					desiredActions[p] = true
				}
			}
		}
	}

	// Deduplicate and Sort
	var actions []string
	for a := range desiredActions {
		actions = append(actions, a)
	}
	sort.Strings(actions)

	// Construct Policy
	policy := PolicyDocument{
		Version: "2012-10-17",
		Statement: []Statement{
			{
				Sid:      "CloudSlashReadOnly",
				Effect:   "Allow",
				Action:   actions,
				Resource: "*",
			},
		},
	}

	return json.MarshalIndent(policy, "", "  ")
}
