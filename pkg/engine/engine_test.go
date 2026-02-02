package engine

import (
	"context"
	"log/slog"
	"testing"
)

func TestEngineInitialization(t *testing.T) {
	// Config is defined in engine.go, so we use it directly here
	cfg := Config{
		Region: "us-east-1",
		Logger: slog.Default(),
	}

	eng, err := New(context.Background(),
		WithConfig(cfg),
		WithLogger(cfg.Logger),
	)

	if err != nil {
		t.Fatalf("Failed to initialize engine: %v", err)
	}

	if eng == nil {
		t.Fatal("Engine instance should not be nil")
	}
}

func TestEngineConfigValidation(t *testing.T) {
	// Test without logger should fail or warn (depending on implementation, here assuming safe defaults)
	eng, err := New(context.Background())
	if err != nil {
		// Start failure is acceptable if config is mandatory
		t.Logf("Engine rejected empty config: %v", err)
	} else {
		if eng.Logger == nil {
			t.Error("Engine should have default logger")
		}
	}
}
