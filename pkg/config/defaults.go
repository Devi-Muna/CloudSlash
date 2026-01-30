package config

// PolicyConfig defines the constraints for the optimization engine.
type PolicyConfig struct {
	MaxChurnPercent float64  // Maximum percentage of infrastructure allowed to change
	MaxSpendLimit   float64  // Maximum budget cap
	AllowedFamilies []string // Whitelisted instance families
}

// RiskConfig defines the parameters for the Bayesian risk engine.
type RiskConfig struct {
	BaselineRisk        float64 // Minimum risk score (0.0 - 1.0)
	DecayFactor         float64 // How fast risk decays over time
	InterruptionPenalty float64 // Risk spike value on failure
}

// Default constants.
const (
	DefaultRegion = "us-east-1"
)

// DefaultPolicyConfig returns the enterprise-safe default policy values.
func DefaultPolicyConfig() PolicyConfig {
	return PolicyConfig{
		MaxChurnPercent: 20.0,
		MaxSpendLimit:   10000.0,
		AllowedFamilies: []string{"t3", "m5", "m6g", "c5", "c6g", "r5", "r6g"},
	}
}

// DefaultRiskConfig returns the standard risk model parameters.
func DefaultRiskConfig() RiskConfig {
	return RiskConfig{
		BaselineRisk:        0.05,
		DecayFactor:         0.95,
		InterruptionPenalty: 1.0,
	}
}
