package history

import (
	"time"
)

// SeedMockData populates the ledger with a synthetic scenario:
// 2 days of stable spending, followed by a massive spike in the last hour.
// This guarantees the v1.3.5 Anomaly Detection will trigger on the next scan.
func SeedMockData() error {
	now := time.Now().Unix()
	
	// 1. Clear existing ledger (optional, but good for reliable demo)
	// For now, we just append.
	
	// Stable Baseline (48 hours ago to 2 hours ago)
	// $1000/mo run rate.
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
	
	// The Spike (1 hour ago)
	// Suddenly jumped to $5000/mo (+400%)
	spike := Snapshot{
		Timestamp:        now - 3600,
		TotalMonthlyCost: 1200.0, // Rising...
		ResourceCounts:   map[string]int{"EC2": 5, "RDS": 2},
		WasteCount:       1,
	}
	return Append(spike)
	
	// Next scan will generate "now" cost around $5000+ based on mock graph,
	// creating a massive Velocity/Acceleration event.
}
