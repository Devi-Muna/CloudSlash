package policy

import (
	"fmt"
	"strings"
)

// Policy defines the "Asimov Constraints" for automated optimization.
type Policy struct {
	MaxChurnPercent float64  // e.g. 20.0
	ForbiddenAZs    []string // e.g. ["us-east-1e"]
	AllowedFamilies []string // e.g. ["m5", "c6g"]
	MaxSpendLimit   float64  // e.g. 5000.0
}

// DefaultPolicy returns a safe baseline.
func DefaultPolicy() Policy {
	return Policy{
		MaxChurnPercent: 20.0,
		ForbiddenAZs:    []string{},
		AllowedFamilies: []string{"t3", "m5", "m6g", "c5", "c6g", "r5", "r6g"},
		MaxSpendLimit:   10000.0,
	}
}

// Validator checks if a proposed action adheres to the policy.
type Validator struct {
	P Policy
}

func NewValidator(p Policy) *Validator {
	return &Validator{P: p}
}

// ValidateProposal checks a proposed optimization plan.
func (v *Validator) ValidateProposal(churnPercent float64, targetInstanceType string, totalCost float64) error {
	// 1. Churn Circuit Breaker
	if churnPercent > v.P.MaxChurnPercent {
		return fmt.Errorf("SAFETY TRIP: Proposed churn %.1f%% exceeds limit %.1f%%", churnPercent, v.P.MaxChurnPercent)
	}

	// 2. Spend Limit
	if totalCost > v.P.MaxSpendLimit {
		return fmt.Errorf("SAFETY TRIP: Total cost $%.2f exceeds limit $%.2f", totalCost, v.P.MaxSpendLimit)
	}

	// 3. Instance Family Whitelist
	allowed := false
	for _, fam := range v.P.AllowedFamilies {
		if strings.HasPrefix(targetInstanceType, fam) {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("SAFETY TRIP: Instance type %s is not in allowed families list", targetInstanceType)
	}

	return nil
}
