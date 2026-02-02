// Package config defines default configuration, policies, and risk parameters.
package config

// PolicyConfig defines the constraints for the optimization engine.
type PolicyConfig struct {
	// MaxChurnPercent is the maximum allowed infrastructure change percentage per run.
	MaxChurnPercent float64
	// MaxSpendLimit is the maximum budget cap.
	MaxSpendLimit float64
	// AllowedFamilies is the list of permitted instance families.
	AllowedFamilies []string
}

// RiskConfig defines the parameters for the Bayesian risk engine.
type RiskConfig struct {
	// BaselineRisk is the minimum risk score (0.0 - 1.0).
	BaselineRisk float64
	// DecayFactor is the risk decay rate over time.
	DecayFactor float64
	// InterruptionPenalty is the risk spike applied upon failure.
	InterruptionPenalty float64
}

// Defaults.
const (
	DefaultRegion = "us-east-1"
)

// DefaultPolicyConfig returns default policy values.
func DefaultPolicyConfig() PolicyConfig {
	return PolicyConfig{
		MaxChurnPercent: 20.0,
		MaxSpendLimit:   10000.0,
		AllowedFamilies: []string{"t3", "m5", "m6g", "c5", "c6g", "r5", "r6g"},
	}
}

// DefaultRiskConfig returns default risk parameters.
func DefaultRiskConfig() RiskConfig {
	return RiskConfig{
		BaselineRisk:        0.05,
		DecayFactor:         0.95,
		InterruptionPenalty: 1.0,
	}
}
