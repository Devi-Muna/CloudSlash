package terraform

import (
	"testing"

	"github.com/DrSkyle/cloudslash/internal/graph"
)

func TestAnalyze_ModuleAwareness(t *testing.T) {
	// 1. Setup Mock Zombies (from "Scan")
	zombies := []*graph.Node{
		{ID: "i-zombie-1", IsWaste: true}, // Belongs to Module A
		{ID: "i-zombie-2", IsWaste: true}, // Belongs to Module A
		{ID: "vol-zombie-3", IsWaste: true}, // Belongs to Module B (Partial)
	}

	// 2. Setup Mock Terraform State
	// Module A: "payments" (2 instances, both zombies) -> Should recommend module deletion
	// Module B: "shared" (2 instances, 1 zombie, 1 safe) -> Should recommend resource deletion
	mockStateJSON := `{
		"resources": [
			{
				"module": "module.payments",
				"type": "aws_instance",
				"name": "worker",
				"mode": "managed",
				"instances": [
					{"attributes": {"id": "i-zombie-1"}},
					{"attributes": {"id": "i-zombie-2"}}
				]
			},
			{
				"module": "module.shared",
				"type": "aws_ebs_volume",
				"name": "data",
				"mode": "managed",
				"instances": [
					{"attributes": {"id": "vol-zombie-3"}},
					{"attributes": {"id": "vol-safe-4"}}
				]
			}
		]
	}`

	state, err := ParseState([]byte(mockStateJSON))
	if err != nil {
		t.Fatalf("Failed to parse mock state: %v", err)
	}

	// 3. Run Analysis
	report := Analyze(zombies, state)

	// 4. Verify Verdict
	
	// Expect Module A to be in ModulesToDelete
	foundMod := false
	for _, m := range report.ModulesToDelete {
		if m == "module.payments" {
			foundMod = true
			break
		}
	}
	if !foundMod {
		t.Errorf("Expected 'module.payments' to be marked for full deletion, but it wasn't. Modules: %v", report.ModulesToDelete)
	}

	// Expect Module B resource to be in ResourcesToDelete
	foundRes := false
	expectedResAddr := "module.shared.aws_ebs_volume.data[0]"
	for _, r := range report.ResourcesToDelete {
		if r == expectedResAddr {
			foundRes = true
			break
		}
	}
	if !foundRes {
		t.Errorf("Expected '%s' to be marked for resource deletion, but it wasn't. Resources: %v", expectedResAddr, report.ResourcesToDelete)
	}

	// Ensure Module B itself is NOT deleted
	for _, m := range report.ModulesToDelete {
		if m == "module.shared" {
			t.Errorf("DANGER: 'module.shared' was marked for full deletion despite having safe resources!")
		}
	}

	// Check Total Zombies
	if report.TotalZombiesFound != 3 {
		t.Errorf("Expected 3 zombies found, got %d", report.TotalZombiesFound)
	}
}
