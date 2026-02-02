package solver

import (
	"fmt"
	"sort"

	"github.com/DrSkyle/cloudslash/v2/pkg/engine/oracle"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/policy"
	"github.com/DrSkyle/cloudslash/v2/pkg/engine/tetris"
)

// InstanceType defines a compute instance configuration.
type InstanceType struct {
	Name       string
	CPU        float64 // mCPU
	RAM        float64 // MiB
	HourlyCost float64
	Region     string
	Zone       string
}

// OptimizationRequest defines solver input parameters.
type OptimizationRequest struct {
	Workloads    []*tetris.Item
	Catalog      []InstanceType
	CurrentSpend float64
}

// AllocationPlan represents the optimized resource allocation.
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

// Solve finds optimal resource allocations.
func (opt *Optimizer) Solve(req OptimizationRequest) (*AllocationPlan, error) {
	var bestPlan *AllocationPlan
	minCost := req.CurrentSpend * 10.0 // Start high

	// Iterate through instance types.

	for _, instance := range req.Catalog {
		// Validate against policy.
		if err := opt.Policy.ValidateProposal(0, instance.Name, 0); err != nil {
			continue
		}

		// Check risk factors.
		risk := opt.Oracle.GetRisk(instance.Zone, instance.Name)
		if risk > 0.5 {
			continue
		}

		// Simulate packing.
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

	// Attempt heterogeneous optimization.
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

// solveHeterogeneous performs multi-phase bin packing.
func (opt *Optimizer) solveHeterogeneous(req OptimizationRequest) (*AllocationPlan, error) {
	// Identify primary workhorse instance.
	candidates := make([]InstanceType, len(req.Catalog))
	copy(candidates, req.Catalog)

	sort.Slice(candidates, func(i, j int) bool {
		// Sort by cost/capacity ratio.
		effI := candidates[i].HourlyCost / (candidates[i].CPU + candidates[i].RAM/1000.0) // Normalize RAM
		effJ := candidates[j].HourlyCost / (candidates[j].CPU + candidates[j].RAM/1000.0)
		return effI < effJ
	})

	if len(candidates) == 0 {
		return nil, fmt.Errorf("empty catalog")
	}
	workhorse := candidates[0]

	// Pack primary workload.
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

	// Check for underutilized tail (dust).
	lastBin := bins[len(bins)-1]

	// Attempt downsizing if utilization < 40%.
	if lastBin.Efficiency() < 0.4 {
		// Separate tail.
		mainFleet := bins[:len(bins)-1]
		dustItems := lastBin.Items

		// Repack tail items.
		// Sort by absolute cost.
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].HourlyCost < candidates[j].HourlyCost
		})

		var dustBin *tetris.Bin

		// Calculate requirements.
		var reqCPU, reqRAM float64
		for _, item := range dustItems {
			reqCPU += item.Dimensions.CPU
			reqRAM += item.Dimensions.RAM
		}

		// Find smallest fit.
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

			// Calculate total cost.
			totalCost := 0.0
			instructions := []string{}

			// Summarize main fleet.
			if len(mainFleet) > 0 {
				totalCost += float64(len(mainFleet)) * workhorse.HourlyCost * 730
				instructions = append(instructions, fmt.Sprintf("pool-main: %d nodes of type %s", len(mainFleet), workhorse.Name))
			}

			// Summarize dust bin.
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

	// Fallback to homogeneous plan.
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
