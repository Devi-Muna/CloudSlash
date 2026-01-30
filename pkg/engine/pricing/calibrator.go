package pricing

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer"
	"github.com/aws/aws-sdk-go-v2/service/costexplorer/types"
)



type DiscountCache struct {
	Factor    float64 `json:"factor"`
	Timestamp int64   `json:"timestamp"`
}

// Calibrator manages cost estimation tuning.
type Calibrator struct {
	logger         *slog.Logger
	cachePath      string
	manualOverride float64
}

func NewCalibrator(logger *slog.Logger, cacheDir string, override float64) *Calibrator {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if cacheDir == "" {
		cacheDir = os.TempDir()
	}
	path := filepath.Join(cacheDir, "discounts.json")
	return &Calibrator{
		logger:         logger,
		cachePath:      path,
		manualOverride: override,
	}
}

// GetDiscountFactor retrieves the calibration factor.
func (c *Calibrator) GetDiscountFactor(ctx context.Context) float64 {
	// Check cache.
	if factor, ok := c.loadCache(); ok {
		return factor
	}

	// Fetch from AWS (fail-open).
	factor, err := c.fetchFromAWS(ctx)
	if err != nil {
		if c.manualOverride > 0 {
			c.logger.Warn("Calibration failed, using manual override", "error", err, "override", c.manualOverride)
			return c.manualOverride
		}
		c.logger.Warn("Calibration failed, using standard list prices", "error", err)
		return 1.0
	}

	// Persist cache.
	c.saveCache(factor)
	return factor
}

func (c *Calibrator) loadCache() (float64, bool) {
	data, err := os.ReadFile(c.cachePath)
	if err != nil {
		return 1.0, false
	}

	var cache DiscountCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return 1.0, false
	}

	// Check TTL (24h).
	if time.Since(time.Unix(cache.Timestamp, 0)) > 24*time.Hour {
		return 1.0, false
	}

	return cache.Factor, true
}

func (c *Calibrator) saveCache(factor float64) {
	cache := DiscountCache{
		Factor:    factor,
		Timestamp: time.Now().Unix(),
	}
	data, _ := json.MarshalIndent(cache, "", "  ")
	os.MkdirAll(filepath.Dir(c.cachePath), 0755)
	os.WriteFile(c.cachePath, data, 0644)
}

func (c *Calibrator) fetchFromAWS(ctx context.Context) (float64, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return 1.0, err
	}

	svc := costexplorer.NewFromConfig(cfg)

	end := time.Now().Format("2006-01-02")
	start := time.Now().AddDate(0, 0, -7).Format("2006-01-02")

	input := &costexplorer.GetCostAndUsageInput{
		TimePeriod: &types.DateInterval{
			Start: aws.String(start),
			End:   aws.String(end),
		},
		Granularity: types.GranularityDaily,
		Metrics:     []string{"AmortizedCost", "UnblendedCost"},
		Filter: &types.Expression{
			Dimensions: &types.DimensionValues{
				Key:    types.DimensionService,
				Values: []string{"Amazon Elastic Compute Cloud - Compute"},
			},
		},
	}

	result, err := svc.GetCostAndUsage(ctx, input)
	if err != nil {
		// Fail-open on error.
		return 1.0, err
	}

	var totalAmortized, totalUnblended float64

	for _, resultByTime := range result.ResultsByTime {
		if amt, ok := resultByTime.Total["AmortizedCost"]; ok {
			val, _ := convertAmount(amt.Amount)
			totalAmortized += val
		}
		if amt, ok := resultByTime.Total["UnblendedCost"]; ok {
			val, _ := convertAmount(amt.Amount)
			totalUnblended += val
		}
	}

	if totalUnblended == 0 {
		return 1.0, nil
	}

	factor := totalAmortized / totalUnblended
	
	// Sanity check factor range.
	if factor > 1.5 || factor < 0.1 {
		return 1.0, nil // Suspicious data
	}

	c.logger.Info("Calibrated Discount Factor", "factor", factor, "source", "aws_cost_explorer")
	return factor, nil
}

func convertAmount(s *string) (float64, error) {
	if s == nil {
		return 0, nil
	}
	// AWS usually returns strings like "123.45"
	return parseFloat(*s), nil
}

func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
