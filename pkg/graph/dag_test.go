package graph

import (
	"testing"
)

func TestMarkWaste_AdvancedSuppression(t *testing.T) {
	nodeCostLow := "arn:low-cost"
	nodeCostHigh := "arn:high-cost"
	nodeJustified := "arn:justified"
	nodeDate := "arn:date"

	g := NewGraph()
	g.AddNode(nodeCostLow, "Test", map[string]interface{}{
		"Tags": map[string]string{"cloudslash:ignore": "cost<10"},
	})
	g.AddNode(nodeCostHigh, "Test", map[string]interface{}{
		"Tags": map[string]string{"cloudslash:ignore": "cost<10"},
	})
	g.AddNode(nodeJustified, "Test", map[string]interface{}{
		"Tags": map[string]string{"cloudslash:ignore": "justified:DisasterRecovery"},
	})
	g.AddNode(nodeDate, "Test", map[string]interface{}{
		"Tags": map[string]string{"cloudslash:ignore": "2099-01-01"},
	})

	// Set Costs manually as they aren't computed here
	// Set Costs manually as they aren't computed here
	g.GetNode(nodeCostLow).Cost = 5.0
	g.GetNode(nodeCostHigh).Cost = 15.0

	// Run MarkWaste
	g.MarkWaste(nodeCostLow, 100)
	g.MarkWaste(nodeCostHigh, 100)
	g.MarkWaste(nodeJustified, 100)
	g.MarkWaste(nodeDate, 100)

	// Assertions

	// 1. Cost < 10 (Cost=5) -> Should be IGNORED (IsWaste=false)
	// 1. Cost < 10 (Cost=5) -> Should be IGNORED (IsWaste=false)
	if g.GetNode(nodeCostLow).IsWaste {
		t.Errorf("Low cost node should satisfy cost<10 and be ignored")
	}

	// 2. Cost < 10 (Cost=15) -> Should be MARKED (IsWaste=true)
	// 2. Cost < 10 (Cost=15) -> Should be MARKED (IsWaste=true)
	if !g.GetNode(nodeCostHigh).IsWaste {
		t.Errorf("High cost node should fail cost<10 and be marked")
	}

	// 3. Justified -> Should be MARKED + JUSTIFIED
	// 3. Justified -> Should be MARKED + JUSTIFIED
	if !g.GetNode(nodeJustified).IsWaste {
		t.Errorf("Justified node should be marked as waste (for tracking)")
	}
	if !g.GetNode(nodeJustified).Justified {
		t.Errorf("Justified node should be flagged Justified=true")
	}
	if g.GetNode(nodeJustified).Justification != "disasterrecovery" {
		t.Errorf("Justification reason mismatch. Got %s", g.GetNode(nodeJustified).Justification)
	}

	// 4. Date -> Should be IGNORED (Future date)
	// 4. Date -> Should be IGNORED (Future date)
	if g.GetNode(nodeDate).IsWaste {
		t.Errorf("Future date snoozed node should be ignored")
	}
}
