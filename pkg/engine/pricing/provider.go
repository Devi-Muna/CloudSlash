package pricing

import (
	"context"
)

// Resource represents a pricing query target.
type Resource struct {
	Type       string            // e.g. "AWS::EC2::Instance"
	Region     string            // e.g. "us-east-1"
	Properties map[string]string // e.g. "InstanceType": "m5.large"
}

// PricingProvider defines the pricing interface.
type PricingProvider interface {
	// GetPrice returns the estimated monthly cost for a resource.
	GetPrice(ctx context.Context, resource *Resource) (float64, error)
	
	// Calibrate calculates the discount factor from real billing data.
	Calibrate(ctx context.Context) error
}
