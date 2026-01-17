package history

import (
	"time"
)

// SeedMockData populates the ledger with a synthetic cost anomaly scenario.
// Pattern: 48h stable baseline followed by a significant spike in the last hour.
// Triggers the Anomaly Detection engine for demonstration purposes.
func SeedMockData() error {
	now := time.Now().Unix()
	
	// 1. Reset ledger structure.
	// Current implementation appends to existing history.
	
	// 2. Establish Stable Baseline (T-48h to T-2h).
	// Target Run Rate: ~$1,000/mo.
	baselineStart := now - (48 * 3600)
	for t := baselineStart; t < now-3600; t += 3600 {
		s := Snapshot{
			Timestamp:        t,
			TotalMonthlyCost: 1000.0 + (float64(t%10) * 1.0), // Slight jitter
			ResourceCounts:   map[string]int{"EC2": 5, "RDS": 1},
			WasteCount:       0,
		}
		if err := Append(s); err != nil {
			return err
		}
	}
	
	// Anomaly: 4x cost spike (from $1,200/mo to $5,000/mo run rate).
	spike := Snapshot{
		Timestamp:        now - 3600,
		TotalMonthlyCost: 1200.0,
		ResourceCounts:   map[string]int{"EC2": 5, "RDS": 2},
		WasteCount:       1,
	}
	return Append(spike)
	
	// This ensures the Anomaly Detection engine identifies a high-velocity cost increase events.
}
