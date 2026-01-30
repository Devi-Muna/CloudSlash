package history

import (
	"fmt"
	"time"
)

// AnalysisResult contains derived cost signals.
type AnalysisResult struct {
	CurrentBurnRate   float64 // $/month
	Velocity          float64 // Velocity is the first derivative (cost change rate).
	Acceleration      float64 // Acceleration is the second derivative (rate of change of velocity).
	
	ProjectedBurn24h  float64 // Projected 24h burn rate.
	TimeToBankrupt    time.Duration
	
	Alerts            []string
}

// Analyze calculates cost trends from historical snapshots.
// Calculate derivatives and budget projections.
func Analyze(history []Snapshot, budget float64) AnalysisResult {
	if len(history) < 2 {
		return AnalysisResult{CurrentBurnRate: 0}
	}

	current := history[len(history)-1]
	prev := history[len(history)-2]

	// Calculate velocity using finite difference.
	timeDelta := float64(current.Timestamp - prev.Timestamp) / 3600.0
	if timeDelta == 0 {
		return AnalysisResult{CurrentBurnRate: current.TotalMonthlyCost}
	}

	costDelta := current.TotalMonthlyCost - prev.TotalMonthlyCost
	velocity := costDelta / timeDelta

	// Calculate acceleration (second-order).
	acceleration := 0.0
	if len(history) >= 3 {
		prev2 := history[len(history)-3]
		timeDelta2 := float64(prev.Timestamp - prev2.Timestamp) / 3600.0
		if timeDelta2 > 0 {
			prevVelocity := (prev.TotalMonthlyCost - prev2.TotalMonthlyCost) / timeDelta2
			acceleration = (velocity - prevVelocity) / timeDelta
		}
	}

	// Project burn using Taylor expansion.
	projectedBurn := current.TotalMonthlyCost + (velocity * 24) + (0.5 * acceleration * 24 * 24)

	// Estimate Time-To-Bankrupt (TTB).
	var ttb time.Duration = -1
	if budget > 0 && velocity > 0 {
		remainingHeadroom := budget - current.TotalMonthlyCost
		if remainingHeadroom > 0 {
			hoursToCap := remainingHeadroom / velocity
			ttb = time.Duration(hoursToCap * float64(time.Hour))
		} else {
			ttb = 0 // Budget exhausted.
		}
	}

	// Generate alerts based on thresholds.
	var alerts []string

	// Check for velocity anomalies.
	if velocity > 1000 {
		alerts = append(alerts, fmt.Sprintf("[CRITICAL] SPEND SPIKE: Spending velocity +$%.0f/mo per hour", velocity))
	}

	// Check for acceleration anomalies.
	if acceleration > 500 {
		alerts = append(alerts, fmt.Sprintf("[WARNING] SPEND ACCELERATION: Spending suggests exponential leak (+%.0f/h²)", acceleration))
	}
	
	// Check for budget exhaustion risks.
	if ttb > 0 && ttb < 24*time.Hour {
		alerts = append(alerts, fmt.Sprintf("[CRITICAL] BUDGET EXHAUSTION: Budget exhaustion predicted in %s", ttb.Round(time.Minute)))
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
