package aws

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

var specsMu sync.RWMutex

// InstanceSpecs defines the compute capacity of an instance type.
type InstanceSpecs struct {
	VCPU   float64 // vCPUs (1 vCPU = 1000 mCPU roughly)
	Memory float64 // MiB
	Arch   string  // "x86_64" or "arm64"
}

// CandidateTypes defines the list of modern instance types to consider for optimization.
// This is a curated list of current generation instances.
var CandidateTypes = []string{
	// General Purpose
	"m5.large", "m5.xlarge", "m5.2xlarge",
	"m6i.large", "m6i.xlarge", "m6i.2xlarge",
	"m6g.large", "m6g.xlarge", "m6g.2xlarge", // Graviton
	"t3.medium", "t3.large", "t3.xlarge", // Burstable

	// Compute Optimized
	"c5.large", "c5.xlarge", "c5.2xlarge",
	"c6i.large", "c6i.xlarge", "c6i.2xlarge",
	"c6g.large", "c6g.xlarge", "c6g.2xlarge",

	// Memory Optimized
	"r5.large", "r5.xlarge", "r5.2xlarge",
	"r6i.large", "r6i.xlarge", "r6i.2xlarge",
	"r6g.large", "r6g.xlarge", "r6g.2xlarge",
}

// specsMap serves as a resilient cache for instance specifications.
// It is dynamically updated via the AWS API (DescribeInstanceTypes) during runtime,
// but retains a curated static catalog to ensure availability during API throttling or offline analysis.
var specsMap = map[string]InstanceSpecs{
	// T3 Family (Burstable)
	"t3.nano":   {VCPU: 2, Memory: 512, Arch: "x86_64"},
	"t3.micro":  {VCPU: 2, Memory: 1024, Arch: "x86_64"},
	"t3.small":  {VCPU: 2, Memory: 2048, Arch: "x86_64"},
	"t3.medium": {VCPU: 2, Memory: 4096, Arch: "x86_64"},
	"t3.large":  {VCPU: 2, Memory: 8192, Arch: "x86_64"},
	"t3.xlarge": {VCPU: 4, Memory: 16384, Arch: "x86_64"},
	"t3.2xlarge":{VCPU: 8, Memory: 32768, Arch: "x86_64"},

	// M5 Family (General Purpose)
	"m5.large":   {VCPU: 2, Memory: 8192, Arch: "x86_64"},
	"m5.xlarge":  {VCPU: 4, Memory: 16384, Arch: "x86_64"},
	"m5.2xlarge": {VCPU: 8, Memory: 32768, Arch: "x86_64"},
	"m5.4xlarge": {VCPU: 16, Memory: 65536, Arch: "x86_64"},
	
	// M6g Family (Graviton2)
	"m6g.medium": {VCPU: 1, Memory: 4096, Arch: "arm64"},
	"m6g.large":  {VCPU: 2, Memory: 8192, Arch: "arm64"},
	"m6g.xlarge": {VCPU: 4, Memory: 16384, Arch: "arm64"},
	"m6g.2xlarge":{VCPU: 8, Memory: 32768, Arch: "arm64"},

	// C5 Family (Compute Optimized)
	"c5.large":   {VCPU: 2, Memory: 4096, Arch: "x86_64"},
	"c5.xlarge":  {VCPU: 4, Memory: 8192, Arch: "x86_64"},
	"c5.2xlarge": {VCPU: 8, Memory: 16384, Arch: "x86_64"},

	// C6g Family (Graviton2)
	"c6g.medium": {VCPU: 1, Memory: 2048, Arch: "arm64"},
	"c6g.large":  {VCPU: 2, Memory: 4096, Arch: "arm64"},
	"c6g.xlarge": {VCPU: 4, Memory: 8192, Arch: "arm64"},
	"c6g.2xlarge":{VCPU: 8, Memory: 16384, Arch: "arm64"},

	// R5 Family (Memory Optimized)
	"r5.large":   {VCPU: 2, Memory: 16384, Arch: "x86_64"},
	"r5.xlarge":  {VCPU: 4, Memory: 32768, Arch: "x86_64"},
	"r5.2xlarge": {VCPU: 8, Memory: 65536, Arch: "x86_64"},
}

// GetSpecs returns the VCPU and Memory for a given instance type.
// Thread-safe: Uses RLock to read from the dynamic cache.
func GetSpecs(instanceType string) InstanceSpecs {
	specsMu.RLock()
	specs, ok := specsMap[instanceType]
	specsMu.RUnlock()

	if ok {
		return specs
	}

	// Heuristic: Infer specs from family or default to a safe baseline.
	// Prevents division-by-zero errors in utility calculations.
	return InstanceSpecs{
		VCPU:   2,
		Memory: 8192,
		Arch:   "x86_64",
	}
}

// UpdateSpecsCache dynamically fetches instance details from AWS to ensure
// the internal catalog is accurate for all observed workloads.
func UpdateSpecsCache(ctx context.Context, client EC2Client, instanceTypes []string) error {
	if len(instanceTypes) == 0 {
		return nil
	}

	// Filter for unknown types to minimize API overhead.
	unique := make(map[string]bool)
	var unknownTypes []types.InstanceType

	specsMu.RLock()
	for _, t := range instanceTypes {
		if _, exists := specsMap[t]; !exists {
			if !unique[t] {
				unknownTypes = append(unknownTypes, types.InstanceType(t))
				unique[t] = true
			}
		}
	}
	specsMu.RUnlock()

	if len(unknownTypes) == 0 {
		return nil
	}

	// Batch fetch details from AWS.
	paginator := ec2.NewDescribeInstanceTypesPaginator(client, &ec2.DescribeInstanceTypesInput{
		InstanceTypes: unknownTypes,
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			// Log failure but safely degrade to static/heuristic mode.
			fmt.Printf("Warning: Failed to sync instance specs for %v: %v\n", unknownTypes, err)
			return err
		}

		specsMu.Lock()
		for _, info := range page.InstanceTypes {
			// Extract vCPU (DefaultVCpus or VCpus)
			vcpu := 0.0
			if info.VCpuInfo != nil && info.VCpuInfo.DefaultVCpus != nil {
				vcpu = float64(*info.VCpuInfo.DefaultVCpus)
			}

			// Extract Memory (SizeInMiB)
			mem := 0.0
			if info.MemoryInfo != nil && info.MemoryInfo.SizeInMiB != nil {
				mem = float64(*info.MemoryInfo.SizeInMiB)
			}

			// Extract Architecture
			arch := "x86_64"
			if len(info.ProcessorInfo.SupportedArchitectures) > 0 {
				// Prefer first reported architecture
				arch = string(info.ProcessorInfo.SupportedArchitectures[0])
			}

			specsMap[string(info.InstanceType)] = InstanceSpecs{
				VCPU:   vcpu,
				Memory: mem,
				Arch:   arch,
			}
		}
		specsMu.Unlock()
	}

	return nil
}

// PricingStrategy defines the interface for determining costs.
type PricingStrategy interface {
	GetEstimatedCost(instanceType, region string) float64
}

// StaticCostEstimator provides fallback pricing when the API is unavailable.
type StaticCostEstimator struct{}

func (s *StaticCostEstimator) GetEstimatedCost(instanceType, region string) float64 {
	// 1. Check family
	if strings.HasPrefix(instanceType, "t") {
		return 30.0 // Cheap burstable
	}
	if strings.HasPrefix(instanceType, "m") {
		if strings.Contains(instanceType, ".xlarge") {
			return 140.0
		}
		return 70.0 // m5.large approx
	}
	if strings.HasPrefix(instanceType, "c") {
		if strings.Contains(instanceType, ".xlarge") {
			return 120.0
		}
		return 60.0
	}
	if strings.HasPrefix(instanceType, "r") {
		if strings.Contains(instanceType, ".xlarge") {
			return 180.0
		}
		return 90.0
	}
	
	// Default Fallback
	return 50.0 
}
