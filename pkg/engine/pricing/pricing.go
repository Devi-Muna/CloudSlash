package pricing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/pricing/types"
)

type PriceRecord struct {
	Price     float64 `json:"price"`
	Timestamp int64   `json:"timestamp"`
}

// Client wraps the AWS Pricing API.
type Client struct {
	logger         *slog.Logger
	svc            *pricing.Client
	cache          map[string]PriceRecord
	mu             sync.RWMutex
	cachePath      string
	ttl            time.Duration
	discountFactor float64
}

// NewClient initializes the pricing client.
// Resolves cache path and defaults.
func NewClient(ctx context.Context, logger *slog.Logger, cacheDir string, manualDiscountRate float64) (*Client, error) {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	if cacheDir == "" {
		cacheDir = os.TempDir()
	}
	os.MkdirAll(cacheDir, 0755)

	// Use us-east-1 for global pricing queries.
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		return nil, err
	}

	cal := NewCalibrator(logger, cacheDir, manualDiscountRate)
	factor := cal.GetDiscountFactor(ctx)

	c := &Client{
		logger:         logger,
		svc:            pricing.NewFromConfig(cfg),
		cache:          make(map[string]PriceRecord),
		cachePath:      filepath.Join(cacheDir, "pricing.json"),
		ttl:            15 * 24 * time.Hour, // 15 Days
		discountFactor: factor,
	}
	
	c.loadCache()
	return c, nil
}

func (c *Client) loadCache() {
	data, err := os.ReadFile(c.cachePath)
	if err == nil {
		json.Unmarshal(data, &c.cache)
	}
}

func (c *Client) saveCache() {
	data, err := json.MarshalIndent(c.cache, "", "  ")
	if err == nil {
		os.WriteFile(c.cachePath, data, 0644)
	}
}

// GetEBSPrice estimates EBS monthly cost.
func (c *Client) GetEBSPrice(ctx context.Context, region, volumeType string, sizeGB int) (float64, error) {
	cacheKey := fmt.Sprintf("ebs-%s-%s", region, volumeType)

	c.mu.RLock()
	record, ok := c.cache[cacheKey]
	c.mu.RUnlock()

	valid := ok && time.Since(time.Unix(record.Timestamp, 0)) < c.ttl

	if !valid {
		var err error
		price, err := c.fetchEBSPrice(ctx, region, volumeType)
		if err != nil {
			return 0, err
		}
		
		c.mu.Lock()
		c.cache[cacheKey] = PriceRecord{Price: price, Timestamp: time.Now().Unix()}
		c.saveCache() // Persist cache.
		c.mu.Unlock()
		
		return price * float64(sizeGB), nil
	}

	return record.Price * float64(sizeGB), nil
}

func (c *Client) fetchEBSPrice(ctx context.Context, region, volumeType string) (float64, error) {
	// Filter products.
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

	// Add volume type.
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
		// Default fallback cost.
		return 0.1, nil
	}

	filters = append(filters, types.Filter{
		Type:  types.FilterTypeTermMatch,
		Field: aws.String("volumeType"),
		Value: aws.String(volTypeVal),
	})

	input := &pricing.GetProductsInput{
		ServiceCode: aws.String("AmazonEC2"),
		Filters:     filters,
		MaxResults:  aws.Int32(1), // Retrieve single match
	}

	out, err := c.svc.GetProducts(ctx, input)
	if err != nil {
		return 0, err
	}

	if len(out.PriceList) == 0 {
		return 0, fmt.Errorf("no pricing found for %s %s", region, volumeType)
	}

	// Parse price.
	return parsePriceFromJSON(out.PriceList[0])
}

// GetEC2InstancePrice estimates EC2 monthly cost.
func (c *Client) GetEC2InstancePrice(ctx context.Context, region, instanceType string) (float64, error) {
	cacheKey := fmt.Sprintf("ec2-%s-%s", region, instanceType)

	c.mu.RLock()
	record, ok := c.cache[cacheKey]
	c.mu.RUnlock()

	valid := ok && time.Since(time.Unix(record.Timestamp, 0)) < c.ttl

	if !valid {
		var err error
		price, err := c.fetchEC2Price(ctx, region, instanceType)
		if err != nil {
			return 0, err
		}
		c.mu.Lock()
		c.cache[cacheKey] = PriceRecord{Price: price, Timestamp: time.Now().Unix()}
		c.saveCache()
		c.mu.Unlock()
		
		return price * 730 * c.discountFactor, nil
	}

	return record.Price * 730 * c.discountFactor, nil // Assumes 730h/month.
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
			Value: aws.String("Linux"),
		},
		{
			Type:  types.FilterTypeTermMatch,
			Field: aws.String("preInstalledSw"),
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
		// Attempt fallback search criteria.
		return 0, fmt.Errorf("no pricing found for %s %s", region, instanceType)
	}

	return parsePriceFromJSON(out.PriceList[0])
}

// GetNATGatewayPrice estimates NAT Gateway monthly cost.
func (c *Client) GetNATGatewayPrice(ctx context.Context, region string) (float64, error) {
	cacheKey := fmt.Sprintf("nat-%s", region)

	c.mu.RLock()
	record, ok := c.cache[cacheKey]
	c.mu.RUnlock()
	
	valid := ok && time.Since(time.Unix(record.Timestamp, 0)) < c.ttl

	if !valid {
		// Short timeout check.
		tCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		var err error
		price, err := c.fetchNATPrice(tCtx, region)
		if err != nil {
			// Default timeout fallback.
			return 0.045 * 730, nil
		}
		c.mu.Lock()
		c.cache[cacheKey] = PriceRecord{Price: price, Timestamp: time.Now().Unix()}
		c.saveCache()
		c.mu.Unlock()
		return price * 730, nil
	}

	return record.Price * 730, nil
}

func (c *Client) fetchNATPrice(ctx context.Context, region string) (float64, error) {
	filters := []types.Filter{
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
			Field: aws.String("productFamily"),
			Value: aws.String("NAT Gateway"),
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
		return 0, fmt.Errorf("no pricing found for NAT Gateway in %s", region)
	}

	return parsePriceFromJSON(out.PriceList[0])
}

// GetEIPPrice estimates unattached EIP monthly cost.
func (c *Client) GetEIPPrice(ctx context.Context, region string) (float64, error) {
	return 0.005 * 730, nil
}

func parsePriceFromJSON(jsonStr string) (float64, error) {
	// Pricing JSON structures.
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
