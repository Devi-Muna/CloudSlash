//go:build e2e

package e2e

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/DrSkyle/cloudslash/pkg/engine/pricing"
)

func TestPricingOverride(t *testing.T) {
	// 1. Setup: Create a temporary config with override
	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// 2. Init Calibrator with 20% discount (0.20)
	cal := pricing.NewCalibrator(logger, tmpDir, 0.20)

	// 3. Execute
	factor := cal.GetDiscountFactor(context.Background())

	// 4. Assert
	if factor != 0.20 {
		t.Fatalf("Expected manual override 0.20, got %.2f", factor)
	}
}
