package history

import (
	"fmt"
	"time"
)

// AnalysisResult holds the derived signals from the history window.
type AnalysisResult struct {
	CurrentBurnRate   float64 // $/month
	Velocity          float64 // $/month per HOUR (The rate of change)
	Acceleration      float64 // $/month per HOUR^2 (The rate of acceleration)
	
	ProjectedBurn24h  float64 // Forecasted burn rate in 24 hours
	TimeToBankrupt    time.Duration
	
	Alerts            []string
}

// Analyze processes the ledger history and returns signal derivatives.
// budget: User defined budget cap (e.g. $10,000). If 0, TTB is skipped.
func Analyze(history []Snapshot, budget float64) AnalysisResult {
	if len(history) < 2 {
		return AnalysisResult{CurrentBurnRate: 0}
	}

	current := history[len(history)-1]
	prev := history[len(history)-2]

	// 1. Calculate Time Delta (Hours)
	timeDelta := float64(current.Timestamp - prev.Timestamp) / 3600.0
	if timeDelta == 0 {
		return AnalysisResult{CurrentBurnRate: current.TotalMonthlyCost}
	}

	// 2. Calculate Velocity (First Derivative): Change in Monthly Cost per Hour
	costDelta := current.TotalMonthlyCost - prev.TotalMonthlyCost
	velocity := costDelta / timeDelta

	// 3. Calculate Acceleration (Second Derivative)
	acceleration := 0.0
	if len(history) >= 3 {
		prev2 := history[len(history)-3]
		timeDelta2 := float64(prev.Timestamp - prev2.Timestamp) / 3600.0
		if timeDelta2 > 0 {
			prevVelocity := (prev.TotalMonthlyCost - prev2.TotalMonthlyCost) / timeDelta2
			acceleration = (velocity - prevVelocity) / timeDelta
		}
	}

	// 4. Project Future Burn (24h)
	projectedBurn := current.TotalMonthlyCost + (velocity * 24) + (0.5 * acceleration * 24 * 24)

	// 5. Time-To-Bankrupt (TTB)
	// Logic: If (CurrentBurn + PredictedGrowth) > Budget, how long until we cross it?
	// Simplified Linear Projection: Limit - Current / Velocity
	var ttb time.Duration = -1
	if budget > 0 && velocity > 0 {
		remainingHeadroom := budget - current.TotalMonthlyCost
		if remainingHeadroom > 0 {
			hoursToCap := remainingHeadroom / velocity
			ttb = time.Duration(hoursToCap * float64(time.Hour))
		} else {
			ttb = 0 // Already bankrupt
		}
	}

	// 6. Generate Alerts
	var alerts []string

	// Velocity Alert (Massive Spike > $1000/mo added in 1 hour)
	if velocity > 1000 {
		alerts = append(alerts, fmt.Sprintf("🚨 SPIKE DETECTED: Spending velocity +$%.0f/mo per hour", velocity))
	}

	// Acceleration Alert (The "Runway Killer")
	if acceleration > 500 {
		alerts = append(alerts, fmt.Sprintf("☢️ ACCELERATION WARNING: Spending suggests exponential leak (+%.0f/h²)", acceleration))
	}
	
	// TTB Alert
	if ttb > 0 && ttb < 24*time.Hour {
		alerts = append(alerts, fmt.Sprintf("💀 RUNWAY CRITICAL: Budget exhaustion predicted in %s", ttb.Round(time.Minute)))
	}

	return AnalysisResult{
		CurrentBurnRate:  current.TotalMonthlyCost,
		Velocity:         velocity,
		Acceleration:     acceleration,
		ProjectedBurn24h: projectedBurn,
		TimeToBankrupt:   ttb,
		Alerts:           alerts,
	}
}
