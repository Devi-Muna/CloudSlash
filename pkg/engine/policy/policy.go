package policy

import (
	"fmt"
	"strings"

	"github.com/DrSkyle/cloudslash/pkg/config"
)

// Policy defines the "Asimov Constraints" for automated optimization.
type Policy struct {
	Config config.PolicyConfig
}

// DefaultPolicy returns a safe baseline.
func DefaultPolicy() Policy {
	return Policy{
		Config: config.DefaultPolicyConfig(),
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
	if churnPercent > v.P.Config.MaxChurnPercent {
		return fmt.Errorf("SAFETY TRIP: Proposed churn %.1f%% exceeds limit %.1f%%", churnPercent, v.P.Config.MaxChurnPercent)
	}

	// 2. Spend Limit
	if totalCost > v.P.Config.MaxSpendLimit {
		return fmt.Errorf("SAFETY TRIP: Total cost $%.2f exceeds limit $%.2f", totalCost, v.P.Config.MaxSpendLimit)
	}

	// 3. Instance Family Whitelist
	allowed := false
	for _, fam := range v.P.Config.AllowedFamilies {
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
