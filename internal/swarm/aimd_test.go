package swarm

import (
	"testing"
	"time"
)

func TestAIMD_Feedback(t *testing.T) {
	aimd := NewAIMD(10, 5, 20)

	// Test Additive Increase
	// Initial state
	if aimd.GetConcurrency() != 10 {
		t.Errorf("Expected initial concurrency 10, got %d", aimd.GetConcurrency())
	}

	// Simulate success (low latency)
	// Need to wait > 100ms because of rate limiting in Feedback
	time.Sleep(110 * time.Millisecond)
	aimd.Feedback(50*time.Millisecond, false)

	if aimd.GetConcurrency() != 15 {
		t.Errorf("Expected concurrency 15 after success, got %d", aimd.GetConcurrency())
	}

	// Test Multiplicative Decrease
	time.Sleep(110 * time.Millisecond)
	aimd.Feedback(500*time.Millisecond, true) // Throttled

	expected := 7 // 15 / 2 = 7
	if aimd.GetConcurrency() != expected {
		t.Errorf("Expected concurrency %d after throttle, got %d", expected, aimd.GetConcurrency())
	}

	// Test Min Limit
	time.Sleep(110 * time.Millisecond)
	aimd.Feedback(500*time.Millisecond, true) // Throttled again -> 3
	time.Sleep(110 * time.Millisecond)
	aimd.Feedback(500*time.Millisecond, true) // Throttled again -> min 5

	if aimd.GetConcurrency() < 5 {
		t.Errorf("Concurrency dropped below min limit: %d", aimd.GetConcurrency())
	}
}
