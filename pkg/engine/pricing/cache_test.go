package pricing

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
)

func TestPricingCache(t *testing.T) {
	// Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "cloudslash_cache_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	cacheFile := filepath.Join(tmpDir, "pricing.json")
	
	// Create dummy AWS config
	cfg, _ := config.LoadDefaultConfig(context.TODO(), config.WithRegion("us-east-1"))

	// Create client manually to inject custom paths
	c := &Client{
		svc:       pricing.NewFromConfig(cfg),
		cache:     make(map[string]PriceRecord),
		mu:        sync.RWMutex{},
		cachePath: cacheFile,
		ttl:       1 * time.Hour,
	}

	// 1. Priming: Manually inject a valid cache entry
	region := "us-east-1"
	instType := "m5.large"
	cacheKey := fmt.Sprintf("ec2-%s-%s", region, instType)
	expectedPrice := 0.096

	c.cache[cacheKey] = PriceRecord{
		Price:     expectedPrice,
		Timestamp: time.Now().Unix(),
	}

	// 2. Test Hit: Should return cached price without calling AWS
	price, err := c.GetEC2InstancePrice(context.Background(), region, instType)
	if err != nil {
		t.Fatalf("Cache hit failed: %v", err)
	}
	
	// Check computation (hourly * 730)
	if price != expectedPrice * 730 {
		t.Errorf("Expected monthly price %.2f, got %.2f", expectedPrice * 730, price)
	}

	// 3. Test Miss/Expiry: Inject expired entry
	expiredKey := "ec2-us-east-1-expired.large"
	c.cache[expiredKey] = PriceRecord{
		Price:     0.50,
		Timestamp: time.Now().Add(-20 * 24 * time.Hour).Unix(), // 20 days old (Over 15 days)
	}
	
	// This should try to fetch from AWS and fail (no creds)
	_, err = c.GetEC2InstancePrice(context.Background(), "us-east-1", "expired.large")
	if err == nil {
		t.Error("Expected error from AWS fetch on cache miss, got nil (Did it use the expired cache?)")
	}

	// 4. Persistence Test
	c.saveCache()
	
	// New client reading same file
	c2 := &Client{
		cache:     make(map[string]PriceRecord),
		cachePath: cacheFile,
	}
	c2.loadCache()
	
	if val, ok := c2.cache[cacheKey]; !ok || val.Price != expectedPrice {
		t.Errorf("Persistence failed. Expected %.4f, got %+v", expectedPrice, val)
	}
}
