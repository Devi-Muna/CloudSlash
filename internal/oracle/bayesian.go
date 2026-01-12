package oracle

import (
	"sync"
)

// RiskEngine tracks the stability of cloud resources.
type RiskEngine struct {
	// Map "Zone:InstanceType" -> RiskScore (0.0 - 1.0)
	// e.g. "us-east-1a:g4dn.xlarge" -> 0.8 (Highly Risky)
	History map[string]float64
	Mu      sync.RWMutex
}

func NewRiskEngine() *RiskEngine {
	return &RiskEngine{
		History: make(map[string]float64),
	}
}

// RecordInterruption spikes the risk for a specific pool to 100%.
func (re *RiskEngine) RecordInterruption(zone, instanceType string) {
	re.Mu.Lock()
	defer re.Mu.Unlock()
	
	key := zone + ":" + instanceType
	re.History[key] = 1.0 // Maximum Pain
}

// Decay applies the healing factor. Should be called periodically (e.g. hourly).
// Using a decay factor of 0.95 means risk halves roughly every 13 steps.
func (re *RiskEngine) Decay(factor float64) {
	re.Mu.Lock()
	defer re.Mu.Unlock()

	for k, v := range re.History {
		// Minimum baseline risk for Spot is never 0.0, maybe 0.05.
		newVal := v * factor
		if newVal < 0.05 {
			newVal = 0.05
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
	return 0.05 // Default baseline
}
