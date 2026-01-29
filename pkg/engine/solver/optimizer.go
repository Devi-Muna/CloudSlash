package solver

import (
	"fmt"
	"sort"

	"github.com/DrSkyle/cloudslash/pkg/engine/oracle"
	"github.com/DrSkyle/cloudslash/pkg/engine/policy"
	"github.com/DrSkyle/cloudslash/pkg/engine/tetris"
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

	// Try all allowed instance types to find the most cost-efficient packing configuration.
	// This simulates packing the entire cluster into each instance type to find the best homogenous fit.
	// and pick the winner. This works well for homogenous clusters.

	for _, instance := range req.Catalog {
		// Check if instance is allowed by policy
		if err := opt.Policy.ValidateProposal(0, instance.Name, 0); err != nil {
			continue
		}

		// Filter out high-risk zones
		risk := opt.Oracle.GetRisk(instance.Zone, instance.Name)
		if risk > 0.5 {
			continue
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

	// Attempt to optimize the remaining efficiency using a mixed-instance strategy.
	heteroPlan, err := opt.solveHeterogeneous(req)
	if err == nil {
		if bestPlan == nil || heteroPlan.TotalCost < minCost {
			bestPlan = heteroPlan
			minCost = heteroPlan.TotalCost
		}
	}

	if bestPlan == nil {
		return nil, fmt.Errorf("no feasible plan found satisfying all constraints")
	}

	return bestPlan, nil
}

// solveHeterogeneous implements a multi-phase bin packing strategy.
func (opt *Optimizer) solveHeterogeneous(req OptimizationRequest) (*AllocationPlan, error) {
	// 1. Identify Workhorse (Best Price/Performance)
	// Sort catalog by efficiency (Cost per unit of compute).
	candidates := make([]InstanceType, len(req.Catalog))
	copy(candidates, req.Catalog)
	
	sort.Slice(candidates, func(i, j int) bool {
		// Minimizing Cost/Capacity (Lower is better)
		effI := candidates[i].HourlyCost / (candidates[i].CPU + candidates[i].RAM/1000.0) // Normalize RAM
		effJ := candidates[j].HourlyCost / (candidates[j].CPU + candidates[j].RAM/1000.0)
		return effI < effJ
	})

	if len(candidates) == 0 {
		return nil, fmt.Errorf("empty catalog")
	}
	workhorse := candidates[0]

	// Pack the primary workload into the most efficient instance type ("main fleet")
	factory := func() *tetris.Bin {
		return &tetris.Bin{
			ID:       fmt.Sprintf("node-%s-main", workhorse.Name),
			Capacity: tetris.Dimensions{CPU: workhorse.CPU, RAM: workhorse.RAM},
		}
	}
	bins := opt.Packer.Pack(req.Workloads, factory)
	
	if len(bins) == 0 {
		return nil, fmt.Errorf("empty workload")
	}

	// Identify the last bin to see if it's underutilized ("dust")
	lastBin := bins[len(bins)-1]
	
	// If last bin is < 40% utilized, try to downsize it.
	if lastBin.Efficiency() < 0.4 {
		// Remove last bin from main list
		mainFleet := bins[:len(bins)-1]
		dustItems := lastBin.Items
		
		// Try to repack the dust items into a smaller, cheaper instance
		// Sort candidates by lowest absolute cost
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].HourlyCost < candidates[j].HourlyCost
		})
		
		var dustBin *tetris.Bin
		
		// Calc required dimensions
		var reqCPU, reqRAM float64
		for _, item := range dustItems {
			reqCPU += item.Dimensions.CPU
			reqRAM += item.Dimensions.RAM
		}

		// Find smallest instance that fits all dust
		for _, cand := range candidates {
			if cand.CPU >= reqCPU && cand.RAM >= reqRAM {
				dustBin = &tetris.Bin{
					ID:       fmt.Sprintf("node-%s-dust", cand.Name),
					Capacity: tetris.Dimensions{CPU: cand.CPU, RAM: cand.RAM},
					Items:    dustItems,
					Used:     tetris.Dimensions{CPU: reqCPU, RAM: reqRAM}, // Approx
				}
				break
			}
		}

		// If we found a better bin, use it.
		if dustBin != nil {
			// Construct Mixed Fleet
			finalBins := append(mainFleet, dustBin)
			
			// Correct logic: Calculate total cost properly.
			totalCost := 0.0
			instructions := []string{}
			
			// Summarize Main Fleet
			if len(mainFleet) > 0 {
				totalCost += float64(len(mainFleet)) * workhorse.HourlyCost * 730
				instructions = append(instructions, fmt.Sprintf("pool-main: %d nodes of type %s", len(mainFleet), workhorse.Name))
			}
			
			// Summarize Dust Bin
			dustCost := 0.0
			dustName := ""
			for _, cand := range candidates {
				if cand.CPU >= reqCPU && cand.RAM >= reqRAM {
					dustCost = cand.HourlyCost * 730
					dustName = cand.Name
					break
				}
			}
			totalCost += dustCost
			instructions = append(instructions, fmt.Sprintf("pool-dust: 1 node of type %s", dustName))

			return &AllocationPlan{
				Nodes:        finalBins,
				TotalCost:    totalCost,
				Savings:      req.CurrentSpend - totalCost,
				RiskScore:    0.1, // Mixed fleet slightly higher risk?
				Instructions: instructions,
			}, nil
		}
	}

	// Fallback to homogenous workhorse plan if optimization failed.
	totalCost := float64(len(bins)) * workhorse.HourlyCost * 730
	return &AllocationPlan{
		Nodes:     bins,
		TotalCost: totalCost,
		Savings:   req.CurrentSpend - totalCost,
		Instructions: []string{
			fmt.Sprintf("Migrate to %d nodes of type %s", len(bins), workhorse.Name),
		},
	}, nil
}
