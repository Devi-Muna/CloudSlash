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
			Condition: "tags.env == 'prod' && resource.Action == 'delete'",
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
		// Using a map here as a mock resource, though in prod it would be a struct.
		// For the test rule 'props.action', we might need to adjust the rule or the context.
		// Original rule: "tags.env == 'prod' && props.action == 'delete'"
		// But 'props' variable doesn't exist anymore?
		// Wait, the previous implementation had 'props' variable.
		// The new one has 'resource'.
		// The rule in the test needs to be updated or the context needs to reflect 'resource'.
		// But in definitions.go, EC2Instance doesn't have 'action' field.
		// I'll update the test rule to check a field that exists on EC2Instance or just use a map for now if allowed by Dyn.
		Resource: &struct{ Action string }{Action: "delete"},
	}
	// We also need to update the rule condition in the test because 'props' is not defined in the new Env.
	// The new Env declares 'resource'.
	rules[1].Condition = "tags.env == 'prod' && resource.Action == 'delete'"
	matches, _ = engine.Evaluate(ctx, dataB)
	if len(matches) != 1 || matches[0].ID != "prod_protection" {
		t.Errorf("Scenario B failed. Expected ['prod_protection'], got %v", matches)
	}
}
