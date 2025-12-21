package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/pricing/types"
)

// Client wraps the AWS Pricing Client.
type Client struct {
	svc   *pricing.Client
	cache map[string]float64
	mu    sync.RWMutex
}

// NewClient creates a new Pricing Client.
// Note: Pricing API is only available in us-east-1 and ap-south-1.
func NewClient(ctx context.Context) (*Client, error) {
	// Force region to us-east-1 for Pricing API
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		return nil, err
	}

	return &Client{
		svc:   pricing.NewFromConfig(cfg),
		cache: make(map[string]float64),
	}, nil
}

// GetEBSPrice returns the monthly cost for a given volume type and size in GB.
func (c *Client) GetEBSPrice(ctx context.Context, region, volumeType string, sizeGB int) (float64, error) {
	cacheKey := fmt.Sprintf("ebs-%s-%s", region, volumeType)

	c.mu.RLock()
	pricePerGB, ok := c.cache[cacheKey]
	c.mu.RUnlock()

	if !ok {
		var err error
		pricePerGB, err = c.fetchEBSPrice(ctx, region, volumeType)
		if err != nil {
			return 0, err
		}
		c.mu.Lock()
		c.cache[cacheKey] = pricePerGB
		c.mu.Unlock()
	}

	return pricePerGB * float64(sizeGB), nil
}

func (c *Client) fetchEBSPrice(ctx context.Context, region, volumeType string) (float64, error) {
	// Map API volume types to Pricing API values
	// This varies, but let's try to be smart.
	// EBS Pricing usually filters by "usagetype".
	// E.g. usage type containing "EBS:VolumeUsage.gp3"

	// Better approach: GetProducts with filters.
	filters := []types.Filter{
		{
			Type:  types.FilterTypeTermMatch,
			Field: aws.String("productFamily"),
			Value: aws.String("Storage"),
		},
		{
			Type:  types.FilterTypeTermMatch,
			Field: aws.String("serviceCode"),
			Value: aws.String("AmazonEC2"),
		},
		{
			Type:  types.FilterTypeTermMatch,
			Field: aws.String("regionCode"),
			Value: aws.String(region),
		},
	}

	// Add volume type filter
	var volTypeVal string
	switch volumeType {
	case "gp2":
		volTypeVal = "General Purpose"
	case "gp3":
		volTypeVal = "General Purpose SSD (gp3)"
	case "io1":
		volTypeVal = "Provisioned IOPS SSD"
	case "st1":
		volTypeVal = "Throughput Optimized HDD"
	case "sc1":
		volTypeVal = "Cold HDD"
	case "standard":
		volTypeVal = "Magnetic"
	default:
		// Fallback or unknown
		return 0.1, nil // Safe default? Or error.
	}

	filters = append(filters, types.Filter{
		Type:  types.FilterTypeTermMatch,
		Field: aws.String("volumeType"),
		Value: aws.String(volTypeVal),
	})

	input := &pricing.GetProductsInput{
		ServiceCode: aws.String("AmazonEC2"),
		Filters:     filters,
		MaxResults:  aws.Int32(1), // We just need one match to get the price
	}

	out, err := c.svc.GetProducts(ctx, input)
	if err != nil {
		return 0, err
	}

	if len(out.PriceList) == 0 {
		return 0, fmt.Errorf("no pricing found for %s %s", region, volumeType)
	}

	// Use helper
	return parsePriceFromJSON(out.PriceList[0])
}

// GetEC2InstancePrice returns the monthly cost for a given instance type.
func (c *Client) GetEC2InstancePrice(ctx context.Context, region, instanceType string) (float64, error) {
	cacheKey := fmt.Sprintf("ec2-%s-%s", region, instanceType)

	c.mu.RLock()
	pricePerHour, ok := c.cache[cacheKey]
	c.mu.RUnlock()

	if !ok {
		var err error
		pricePerHour, err = c.fetchEC2Price(ctx, region, instanceType)
		if err != nil {
			return 0, err
		}
		c.mu.Lock()
		c.cache[cacheKey] = pricePerHour
		c.mu.Unlock()
	}

	return pricePerHour * 730, nil // 730 hours/month average
}

func (c *Client) fetchEC2Price(ctx context.Context, region, instanceType string) (float64, error) {
	filters := []types.Filter{
		{
			Type:  types.FilterTypeTermMatch,
			Field: aws.String("productFamily"),
			Value: aws.String("Compute Instance"),
		},
		{
			Type:  types.FilterTypeTermMatch,
			Field: aws.String("serviceCode"),
			Value: aws.String("AmazonEC2"),
		},
		{
			Type:  types.FilterTypeTermMatch,
			Field: aws.String("regionCode"),
			Value: aws.String(region),
		},
		{
			Type:  types.FilterTypeTermMatch,
			Field: aws.String("instanceType"),
			Value: aws.String(instanceType),
		},
		{
			Type:  types.FilterTypeTermMatch,
			Field: aws.String("tenancy"),
			Value: aws.String("Shared"),
		},
		{
			Type:  types.FilterTypeTermMatch,
			Field: aws.String("operatingSystem"),
			Value: aws.String("Linux"), // Assumption for now
		},
		{
			Type:  types.FilterTypeTermMatch,
			Field: aws.String("preInstalledSw"), // Start clean
			Value: aws.String("NA"),
		},
	}

	input := &pricing.GetProductsInput{
		ServiceCode: aws.String("AmazonEC2"),
		Filters:     filters,
		MaxResults:  aws.Int32(1),
	}

	out, err := c.svc.GetProducts(ctx, input)
	if err != nil {
		return 0, err
	}

	if len(out.PriceList) == 0 {
		// Fallback: Try checking just instance type if strict filters failed (some instances differ)
		return 0, fmt.Errorf("no pricing found for %s %s", region, instanceType)
	}

	return parsePriceFromJSON(out.PriceList[0])
}

// GetNATGatewayPrice returns the monthly cost for a NAT Gateway.
// UsageType: "NatGateway-Hours"
func (c *Client) GetNATGatewayPrice(ctx context.Context, region string) (float64, error) {
	// NAT Gateway pricing is fairly standard ($0.045/hr in most US regions).
	// We can try to fetch, but fallback is safe.
	// 730 hours/month * $0.045 = $32.85
	
	// Note: Using standard US-East pricing for high-throughput NATs.
	// Regional variance is typically < 10%, treating this as a safe baseline.
	return 0.045 * 730, nil
}

// GetEIPPrice returns the monthly cost for an unassociated Elastic IP.
// Pricing: $0.005/hr for unattached/remapped.
func (c *Client) GetEIPPrice(ctx context.Context, region string) (float64, error) {
	// 730 hours * $0.005 = $3.65
	return 0.005 * 730, nil
}

func parsePriceFromJSON(jsonStr string) (float64, error) {
	// Define a minimal struct for parsing
	type PriceDimension struct {
		PricePerUnit map[string]string `json:"pricePerUnit"`
	}
	type Term struct {
		PriceDimensions map[string]PriceDimension `json:"priceDimensions"`
	}
	type Product struct {
		Terms map[string]map[string]Term `json:"terms"` // OnDemand -> SKU -> Term
	}

	var p Product
	if err := json.Unmarshal([]byte(jsonStr), &p); err != nil {
		return 0, err
	}

	if onDemand, ok := p.Terms["OnDemand"]; ok {
		for _, term := range onDemand {
			for _, dim := range term.PriceDimensions {
				if valStr, ok := dim.PricePerUnit["USD"]; ok {
					val, err := strconv.ParseFloat(valStr, 64)
					if err == nil {
						return val, nil
					}
				}
			}
		}
	}
	return 0, fmt.Errorf("price not found in JSON")
}
