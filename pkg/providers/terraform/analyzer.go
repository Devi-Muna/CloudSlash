package terraform

import (
	"encoding/json"
	"fmt"

	"github.com/DrSkyle/cloudslash/pkg/graph"
)

// AnalysisReport holds state analysis results.
type AnalysisReport struct {
	ModulesToDelete   []string // List of modules to remove.
	ResourcesToDelete []string // List of resources to remove.
	TotalUnused       int
}

// Analyze compares unused nodes with Terraform state.
// Identifies unused modules and resources.
func Analyze(unused []*graph.Node, state *TerraformState) *AnalysisReport {
	report := &AnalysisReport{
		ModulesToDelete:   []string{},
		ResourcesToDelete: []string{},
	}

	// 1. Index unused resources.
	// Match by ID or ARN.
	unusedMap := make(map[string]bool)
	for _, z := range unused {
		unusedMap[z.ID] = true
	}

	// 2. Calculate module usage statistics.
	
	moduleTotal := make(map[string]int)
	
	moduleUnused := make(map[string]int)
	
	moduleUnusedAddrs := make(map[string][]string)

	// Iterate through state resources.
	for _, res := range state.Resources {
		// Only care about managed resources, not data sources
		if res.Mode != "managed" {
			continue
		}

		addrBase := getAddressBase(res) // e.g., "module.vpc.aws_subnet.private"
		modulePath := getModulePath(res) // e.g., "module.vpc"

		// Track Total Resources per module
		if modulePath != "" {
			countInstances(&res, func() { moduleTotal[modulePath]++ })
		}

		// Check for unused instances.
		for i, inst := range res.Instances {
			// Extract resource identifiers.
			var attrs ParsedAttribute
			if err := json.Unmarshal(inst.Attributes, &attrs); err != nil {
				continue
			}

			// Determine if unused.
			isUnused := false
			if attrs.ID != "" && unusedMap[attrs.ID] {
				isUnused = true
			} else if attrs.ARN != "" && unusedMap[attrs.ARN] {
				isUnused = true
			}

			if isUnused {
				// Build resource address.
				fullAddr := addrBase
				// Handle indexed resources.
				
				
				
				if len(res.Instances) > 1 {
					fullAddr = fmt.Sprintf("%s[%d]", addrBase, i)
				}

				if modulePath != "" {
					moduleUnused[modulePath]++
					moduleUnusedAddrs[modulePath] = append(moduleUnusedAddrs[modulePath], fullAddr)
				} else {
					// Add root resource to deletion list.
					report.ResourcesToDelete = append(report.ResourcesToDelete, fullAddr)
				}
				report.TotalUnused++
			}
		}
	}

	// 3. Aggregate module results.
	for mod, total := range moduleTotal {
		unusedCount := moduleUnused[mod]
		
		// If NO unused resources in this module, skip
		if unusedCount == 0 {
			continue
		}

		// Check if entire module is unused.
		if unusedCount == total {
			report.ModulesToDelete = append(report.ModulesToDelete, mod)
		} else {
			// Add individual resources from partial modules.
			report.ResourcesToDelete = append(report.ResourcesToDelete, moduleUnusedAddrs[mod]...)
		}
	}

	return report
}

// Helpers

func getAddressBase(res Resource) string {
	base := fmt.Sprintf("%s.%s", res.Type, res.Name)
	if res.Module != "" {
		return fmt.Sprintf("%s.%s", res.Module, base)
	}
	return base
}

func getModulePath(res Resource) string {
	return res.Module
}

func countInstances(res *Resource, inc func()) {
	for range res.Instances {
		inc()
	}
}
