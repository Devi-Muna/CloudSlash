package oracle

import (
	"sync"

	"github.com/DrSkyle/cloudslash/pkg/config"
)

// RiskEngine models resource stability and preemption risk.
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

// RecordInterruption increases risk for a pool.
func (re *RiskEngine) RecordInterruption(zone, instanceType string) {
	re.Mu.Lock()
	defer re.Mu.Unlock()
	
	key := zone + ":" + instanceType
	re.History[key] = re.Config.InterruptionPenalty
}

// Decay normalizes risk over time.
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

// GetRisk queries the current risk probability.
func (re *RiskEngine) GetRisk(zone, instanceType string) float64 {
	re.Mu.RLock()
	defer re.Mu.RUnlock()

	key := zone + ":" + instanceType
	if val, ok := re.History[key]; ok {
		return val
	}
	return re.Config.BaselineRisk
}
