package solver

import (
	"fmt"
	"testing"

	"github.com/DrSkyle/cloudslash/pkg/config"
	"github.com/DrSkyle/cloudslash/pkg/engine/oracle"
	"github.com/DrSkyle/cloudslash/pkg/engine/policy"
	"github.com/DrSkyle/cloudslash/pkg/engine/tetris"
)

func TestSolveHeterogeneous(t *testing.T) {
	// 1. Setup
	// "Small": $1/hr, 1 CPU
	// "Large": $10/hr, 12 CPU (Efficiency: $0.83/CPU) -> Workhorse
	
	// Scenario: Need 13 CPUs.
	// Homogenous Small: 13 * $1 = $13
	// Homogenous Large: 2 * $10 = $20 (Capacity 24, Used 13)
	// Heterogenous: 1 Large ($10) + 1 Small ($1) = $11 -> WINNER

	catalog := []InstanceType{
		{Name: "small", CPU: 1000, RAM: 1024, HourlyCost: 1.0, Region: "us-east-1", Zone: "us-east-1a"},
		{Name: "large", CPU: 12000, RAM: 12288, HourlyCost: 10.0, Region: "us-east-1", Zone: "us-east-1a"},
	}

	workloads := []*tetris.Item{}
	for i := 0; i < 13; i++ {
		workloads = append(workloads, &tetris.Item{
			ID: fmt.Sprintf("pod-%d", i),
			Dimensions: tetris.Dimensions{CPU: 1000, RAM: 1024},
		})
	}

	req := OptimizationRequest{
		Workloads:    workloads,
		Catalog:      catalog,
		CurrentSpend: 100.0,
	}

	// Mock dependencies
	oracleEngine := oracle.NewRiskEngine(config.RiskConfig{}) // No client needed for basic risk
	policyEngine := &policy.Validator{}

	opt := NewOptimizer(oracleEngine, policyEngine)

	// 2. Execute
	plan, err := opt.Solve(req)
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	// 3. Assert
	// Expecting TotalCost $11 * 730 = $8030
	expectedHourly := 11.0
	actualHourly := plan.TotalCost / 730.0

	t.Logf("Plan Instructions:\n")
	for _, instr := range plan.Instructions {
		t.Logf(" - %s\n", instr)
	}

	if actualHourly != expectedHourly {
		t.Errorf("Expected Hourly Cost $%.2f, got $%.2f", expectedHourly, actualHourly)
	}

	if len(plan.Nodes) != 2 {
		t.Errorf("Expected 2 nodes, got %d", len(plan.Nodes))
	}
}

func TestSolveHomogenousFallback(t *testing.T) {
	// Scenario: Need 10 CPUs.
	// Large holds 12. 
	// Hetero logic should just pick 1 Large ($10).
	// Small ($1) x 10 = $10.
	// Tie. But Large is "Workhorse" so it might win on preference or efficiency first.
	
	// Let's make Large $9.0 (Cheaper)
	// Small $1.0
	// 10 Items.
	// Homogenous Small: $10.
	// Homogenous Large: $9.
	// Hetero Phase 1 (Pack into Large): Matches Homogenous Large.

	catalog := []InstanceType{
		{Name: "small", CPU: 1000, RAM: 1024, HourlyCost: 1.0, Region: "us-east-1", Zone: "us-east-1a"},
		{Name: "large", CPU: 12000, RAM: 12288, HourlyCost: 9.0, Region: "us-east-1", Zone: "us-east-1a"},
	}

	workloads := []*tetris.Item{}
	for i := 0; i < 10; i++ {
		workloads = append(workloads, &tetris.Item{
			ID: fmt.Sprintf("pod-%d", i),
			Dimensions: tetris.Dimensions{CPU: 1000, RAM: 1024},
		})
	}

	req := OptimizationRequest{
		Workloads:    workloads,
		Catalog:      catalog,
		CurrentSpend: 100.0,
	}

	opt := NewOptimizer(oracle.NewRiskEngine(config.RiskConfig{}), &policy.Validator{})
	plan, err := opt.Solve(req)
	if err != nil {
		t.Fatalf("Solve failed: %v", err)
	}

	if plan.TotalCost/730.0 != 9.0 {
		t.Errorf("Expected $9.00, got $%.2f", plan.TotalCost/730.0)
	}
}
