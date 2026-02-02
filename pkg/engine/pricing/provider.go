package pricing

import (
	"context"
)

// Resource represents a pricing query target.
type Resource struct {
	Type       string
	Region     string
	Properties map[string]string
}

// PricingProvider defines the pricing interface.
type PricingProvider interface {
	// GetPrice returns the estimated monthly cost for a resource.
	GetPrice(ctx context.Context, resource *Resource) (float64, error)

	// Calibrate calculates the discount factor from real billing data.
	Calibrate(ctx context.Context) error
}
