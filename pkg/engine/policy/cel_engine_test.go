package policy

import (
	"context"
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
			Condition: "cost > 1000.0",
			Action:    "alert",
		},
		{
			ID:        "prod_protection",
			Condition: "tags.env == 'prod' && props.action == 'delete'",
			Action:    "block",
		},
	}

	// 3. Compile
	if err := engine.Compile(rules); err != nil {
		t.Fatalf("Compilation failed: %v", err)
	}

	// 4. Evaluate Scenario A: High Cost
	ctx := context.Background()
	dataA := EvaluationContext{
		Cost: 1500,
		Tags: map[string]string{"env": "dev"},
	}
	matches, _ := engine.Evaluate(ctx, dataA)
	if len(matches) != 1 || matches[0].ID != "high_cost" {
		t.Errorf("Scenario A failed. Expected ['high_cost'], got %v", matches)
	}

	// 5. Evaluate Scenario B: Protected
	dataB := EvaluationContext{
		Cost: 50,
		Tags: map[string]string{"env": "prod"},
		Props: map[string]interface{}{"action": "delete"},
	}
	matches, _ = engine.Evaluate(ctx, dataB)
	if len(matches) != 1 || matches[0].ID != "prod_protection" {
		t.Errorf("Scenario B failed. Expected ['prod_protection'], got %v", matches)
	}
}
