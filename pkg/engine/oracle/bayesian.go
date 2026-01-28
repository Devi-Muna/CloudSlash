package oracle

import (
	"sync"

	"github.com/DrSkyle/cloudslash/pkg/config"
)

// RiskEngine tracks the stability of cloud resources.
type RiskEngine struct {
	Config  config.RiskConfig
	History map[string]float64
	Mu      sync.RWMutex
}

func NewRiskEngine(cfg config.RiskConfig) *RiskEngine {
	return &RiskEngine{
		Config:  cfg,
		History: make(map[string]float64),
	}
}

// RecordInterruption spikes the risk for a specific pool.
func (re *RiskEngine) RecordInterruption(zone, instanceType string) {
	re.Mu.Lock()
	defer re.Mu.Unlock()
	
	key := zone + ":" + instanceType
	re.History[key] = re.Config.InterruptionPenalty // Use configured penalty (e.g. 1.0)
}

// Decay applies the healing factor.
func (re *RiskEngine) Decay() {
	re.Mu.Lock()
	defer re.Mu.Unlock()

	for k, v := range re.History {
		newVal := v * re.Config.DecayFactor
		if newVal < re.Config.BaselineRisk {
			newVal = re.Config.BaselineRisk
		}
		re.History[k] = newVal
	}
}

// GetRisk returns the current probability of interruption.
func (re *RiskEngine) GetRisk(zone, instanceType string) float64 {
	re.Mu.RLock()
	defer re.Mu.RUnlock()

	key := zone + ":" + instanceType
	if val, ok := re.History[key]; ok {
		return val
	}
	return re.Config.BaselineRisk
}
