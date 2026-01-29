package policy

import (
	"testing"
)

func TestCELEngine(t *testing.T) {
	// 1. Initialize Engine
	engine, err := NewCELEngine()
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// 2. Define Rules
	rules := []DynamicRule{
		{
			ID:        "high_cost",
			Condition: "input.cost > 1000",
			Action:    "alert",
		},
		{
			ID:        "prod_protection",
			Condition: "input.tags.env == 'prod' && input.action == 'delete'",
			Action:    "block",
		},
	}

	// 3. Compile
	if err := engine.Compile(rules); err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}

	// 4. Evaluate Scenario A: High Cost
	dataA := map[string]interface{}{
		"cost": 1500,
		"tags": map[string]interface{}{"env": "dev"},
	}
	matches, _ := engine.Evaluate(dataA)
	if len(matches) != 1 || matches[0] != "high_cost" {
		t.Errorf("Scenario A failed. Expected ['high_cost'], got %v", matches)
	}

	// 5. Evaluate Scenario B: Protected
	dataB := map[string]interface{}{
		"cost":   50,
		"action": "delete",
		"tags":   map[string]interface{}{"env": "prod"},
	}
	matches, _ = engine.Evaluate(dataB)
	if len(matches) != 1 || matches[0] != "prod_protection" {
		t.Errorf("Scenario B failed. Expected ['prod_protection'], got %v", matches)
	}
}
