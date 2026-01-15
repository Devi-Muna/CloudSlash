package solver

import (
	"fmt"

	"github.com/DrSkyle/cloudslash/internal/oracle"
	"github.com/DrSkyle/cloudslash/internal/policy"
	"github.com/DrSkyle/cloudslash/internal/tetris"
)

// InstanceType represents a purchasable compute unit.
type InstanceType struct {
	Name       string
	CPU        float64 // mCPU
	RAM        float64 // MiB
	HourlyCost float64
	Region     string
	Zone       string
}

// OptimizationRequest encapsulates the parameters for a solver run.
type OptimizationRequest struct {
	Workloads    []*tetris.Item
	Catalog      []InstanceType
	CurrentSpend float64
}

// AllocationPlan represents the solver's recommended state.
type AllocationPlan struct {
	Nodes        []*tetris.Bin
	TotalCost    float64
	Savings      float64
	RiskScore    float64
	Instructions []string
}

type Optimizer struct {
	Oracle *oracle.RiskEngine
	Policy *policy.Validator
	Packer *tetris.Packer
}

func NewOptimizer(o *oracle.RiskEngine, p *policy.Validator) *Optimizer {
	return &Optimizer{
		Oracle: o,
		Policy: p,
		Packer: tetris.NewPacker(),
	}
}

// Solve generates the optimal allocation plan.
// It iterates through available instance families to find the most cost-efficient packing configuration that satisfies policy and risk constraints.
func (opt *Optimizer) Solve(req OptimizationRequest) (*AllocationPlan, error) {
	var bestPlan *AllocationPlan
	minCost := req.CurrentSpend * 10.0 // Start high

	// Strategy: Implements a brute-force heuristic over allowed instance families.
	// While a MILP solver provides mathematical optimality, this simulation approach
	// we simulate packing the entire cluster into each allowed instance type
	// and pick the winner. This works well for homogenous clusters.

	for _, instance := range req.Catalog {
		// 1. Policy Check: Is this instance allowed?
		if err := opt.Policy.ValidateProposal(0, instance.Name, 0); err != nil {
			continue // Skip forbidden instances
		}

		// 2. Risk Check: Is the zone/instance safe?
		risk := opt.Oracle.GetRisk(instance.Zone, instance.Name)
		if risk > 0.5 {
			continue // Skip high-risk pools
		}

		// 3. Simulation: Pack everything into this instance type.
		// Simulation assumes homogenous packing for MVP iteration.
		factory := func() *tetris.Bin {
			return &tetris.Bin{
				ID:       fmt.Sprintf("node-%s-gen", instance.Name),
				Capacity: tetris.Dimensions{CPU: instance.CPU, RAM: instance.RAM},
			}
		}

		bins := opt.Packer.Pack(req.Workloads, factory)
		
		totalCost := float64(len(bins)) * instance.HourlyCost * 730 // Monthly
		
		if totalCost < minCost {
			minCost = totalCost
			bestPlan = &AllocationPlan{
				Nodes:     bins,
				TotalCost: totalCost,
				Savings:   req.CurrentSpend - totalCost,
				RiskScore: risk,
				Instructions: []string{
					fmt.Sprintf("Migrate to %d nodes of type %s", len(bins), instance.Name),
				},
			}
		}
	}

	if bestPlan == nil {
		return nil, fmt.Errorf("no feasible plan found satisfying all constraints")
	}

	return bestPlan, nil
}
