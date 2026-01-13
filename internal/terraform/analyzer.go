package terraform

import (
	"encoding/json"
	"fmt"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

// AnalysisReport contains the results of the Terraform state analysis.
type AnalysisReport struct {
	ModulesToDelete   []string // e.g. "module.payments"
	ResourcesToDelete []string // e.g. "module.shared.aws_s3_bucket.logs"
	TotalUnused       int
}

// Analyze correlates confirmed unused nodes from the scan with the Terraform state.
// It implements the "Module Awareness" logic: if all resources in a module are unused,
// recommend deleting the module.
func Analyze(unused []*graph.Node, state *TerraformState) *AnalysisReport {
	report := &AnalysisReport{
		ModulesToDelete:   []string{},
		ResourcesToDelete: []string{},
	}

	// 1. Index Unused Resources for O(1) matching
	// We match against ID (mostly) and ARN (fallback).
	unusedMap := make(map[string]bool)
	for _, z := range unused {
		unusedMap[z.ID] = true
		// Additional ID forms? For now, we rely on the primary ID (i-xxx, vol-xxx).
	}

	// 2. Build Module Stats
	// Map: ModulePath -> Total Resource Count
	moduleTotal := make(map[string]int)
	// Map: ModulePath -> Unused Resource Count
	moduleUnused := make(map[string]int)
	// Map: ModulePath -> List of Unused Resource Clean Addresses (for fallback)
	moduleUnusedAddrs := make(map[string][]string)

	// Traverse the State
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

		// Check Instances for Zombie DNA Match
		for i, inst := range res.Instances {
			// Extract ID/ARN from attributes bag
			var attrs ParsedAttribute
			if err := json.Unmarshal(inst.Attributes, &attrs); err != nil {
				continue
			}

			// Match Logic
			isUnused := false
			if attrs.ID != "" && unusedMap[attrs.ID] {
				isUnused = true
			} else if attrs.ARN != "" && unusedMap[attrs.ARN] {
				isUnused = true
			}

			if isUnused {
				// Construct the full address for this instance
				fullAddr := addrBase
				// If multiple instances allow index, usually we'd append [i],
				// but simplistic approach: standard address matching
				// Ideally we'd handle "aws_instance.web[0]", but for "State Analysis"
				// we'll stick to base address if count=1.
				if len(res.Instances) > 1 {
					fullAddr = fmt.Sprintf("%s[%d]", addrBase, i)
				}

				if modulePath != "" {
					moduleUnused[modulePath]++
					moduleUnusedAddrs[modulePath] = append(moduleUnusedAddrs[modulePath], fullAddr)
				} else {
					// Root module resource -> Direct delete
					report.ResourcesToDelete = append(report.ResourcesToDelete, fullAddr)
				}
				report.TotalUnused++
			}
		}
	}

	// 3. The Verdict: Module Aggregation Logic
	for mod, total := range moduleTotal {
		unusedCount := moduleUnused[mod]
		
		// If NO unused resources in this module, skip
		if unusedCount == 0 {
			continue
		}

		// Logic: 100% Unused Rate?
		if unusedCount == total {
			report.ModulesToDelete = append(report.ModulesToDelete, mod)
		} else {
			// Partial unused: Fallback to individual resources
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
