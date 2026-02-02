package config

import (
	"testing"
)

func TestDefaultPolicyConfig(t *testing.T) {
	config := DefaultPolicyConfig()

	if config.MaxChurnPercent != 20.0 {
		t.Errorf("Expected MaxChurnPercent 20.0, got %f", config.MaxChurnPercent)
	}

	if config.MaxSpendLimit != 10000.0 {
		t.Errorf("Expected MaxSpendLimit 10000.0, got %f", config.MaxSpendLimit)
	}

	foundM5 := false
	for _, fam := range config.AllowedFamilies {
		if fam == "m5" {
			foundM5 = true
			break
		}
	}
	if !foundM5 {
		t.Error("Expected 'm5' to be in AllowedFamilies")
	}
}

func TestDefaultRiskConfig(t *testing.T) {
	config := DefaultRiskConfig()

	if config.BaselineRisk != 0.05 {
		t.Errorf("Expected BaselineRisk 0.05, got %f", config.BaselineRisk)
	}

	if config.DecayFactor >= 1.0 {
		t.Error("DecayFactor must be less than 1.0 to ensure convergence")
	}
}
