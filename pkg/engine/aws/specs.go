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

// InstanceSpecs contains hardware specifications.
type InstanceSpecs struct {
	VCPU   float64 // vCPUs (1 vCPU = 1000 mCPU roughly)
	Memory float64 // MiB
	Arch   string  // "x86_64" or "arm64"
}

// CandidateTypes is a list of modern instance types to prefer.
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

// specsMap is an in-memory cache of instance specifications.
var specsMap = map[string]InstanceSpecs{
	// T3 (Burstable).
	"t3.nano":    {VCPU: 2, Memory: 512, Arch: "x86_64"},
	"t3.micro":   {VCPU: 2, Memory: 1024, Arch: "x86_64"},
	"t3.small":   {VCPU: 2, Memory: 2048, Arch: "x86_64"},
	"t3.medium":  {VCPU: 2, Memory: 4096, Arch: "x86_64"},
	"t3.large":   {VCPU: 2, Memory: 8192, Arch: "x86_64"},
	"t3.xlarge":  {VCPU: 4, Memory: 16384, Arch: "x86_64"},
	"t3.2xlarge": {VCPU: 8, Memory: 32768, Arch: "x86_64"},

	// M5 (GP).
	"m5.large":   {VCPU: 2, Memory: 8192, Arch: "x86_64"},
	"m5.xlarge":  {VCPU: 4, Memory: 16384, Arch: "x86_64"},
	"m5.2xlarge": {VCPU: 8, Memory: 32768, Arch: "x86_64"},
	"m5.4xlarge": {VCPU: 16, Memory: 65536, Arch: "x86_64"},

	// M6g (Graviton).
	"m6g.medium":  {VCPU: 1, Memory: 4096, Arch: "arm64"},
	"m6g.large":   {VCPU: 2, Memory: 8192, Arch: "arm64"},
	"m6g.xlarge":  {VCPU: 4, Memory: 16384, Arch: "arm64"},
	"m6g.2xlarge": {VCPU: 8, Memory: 32768, Arch: "arm64"},

	// C5 (Compute).
	"c5.large":   {VCPU: 2, Memory: 4096, Arch: "x86_64"},
	"c5.xlarge":  {VCPU: 4, Memory: 8192, Arch: "x86_64"},
	"c5.2xlarge": {VCPU: 8, Memory: 16384, Arch: "x86_64"},

	// C6g (Graviton).
	"c6g.medium":  {VCPU: 1, Memory: 2048, Arch: "arm64"},
	"c6g.large":   {VCPU: 2, Memory: 4096, Arch: "arm64"},
	"c6g.xlarge":  {VCPU: 4, Memory: 8192, Arch: "arm64"},
	"c6g.2xlarge": {VCPU: 8, Memory: 16384, Arch: "arm64"},

	// R5 (Memory).
	"r5.large":   {VCPU: 2, Memory: 16384, Arch: "x86_64"},
	"r5.xlarge":  {VCPU: 4, Memory: 32768, Arch: "x86_64"},
	"r5.2xlarge": {VCPU: 8, Memory: 65536, Arch: "x86_64"},
}

// GetSpecs retrieves instance specifications, falling back to a baseline if unknown.
func GetSpecs(instanceType string) InstanceSpecs {
	specsMu.RLock()
	specs, ok := specsMap[instanceType]
	specsMu.RUnlock()

	if ok {
		return specs
	}

	// Return fallback baseline if type is unknown.
	return InstanceSpecs{
		VCPU:   2,
		Memory: 8192,
		Arch:   "x86_64",
	}
}

// UpdateSpecsCache populates the specification cache.
func UpdateSpecsCache(ctx context.Context, client EC2Client, instanceTypes []string) error {
	if len(instanceTypes) == 0 {
		return nil
	}

	// Filter out types that are already cached.
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

	// Batch fetch.
	paginator := ec2.NewDescribeInstanceTypesPaginator(client, &ec2.DescribeInstanceTypesInput{
		InstanceTypes: unknownTypes,
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			// Log warning on failure but do not halt execution.
			fmt.Printf("Warning: Failed to sync instance specs for %v: %v\n", unknownTypes, err)
			return err
		}

		specsMu.Lock()
		for _, info := range page.InstanceTypes {
			// vCPU.
			vcpu := 0.0
			if info.VCpuInfo != nil && info.VCpuInfo.DefaultVCpus != nil {
				vcpu = float64(*info.VCpuInfo.DefaultVCpus)
			}

			// Memory.
			mem := 0.0
			if info.MemoryInfo != nil && info.MemoryInfo.SizeInMiB != nil {
				mem = float64(*info.MemoryInfo.SizeInMiB)
			}

			// Arch.
			arch := "x86_64"
			if len(info.ProcessorInfo.SupportedArchitectures) > 0 {
				// Prefer first.
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

// PricingStrategy defines the interface for retrieving cost estimates.
type PricingStrategy interface {
	GetEstimatedCost(instanceType, region string) float64
}

// StaticCostEstimator provides rough cost estimates.
type StaticCostEstimator struct{}

func (s *StaticCostEstimator) GetEstimatedCost(instanceType, region string) float64 {
	// Check family.
	if strings.HasPrefix(instanceType, "t") {
		return 30.0 // Cheap.
	}
	if strings.HasPrefix(instanceType, "m") {
		if strings.Contains(instanceType, ".xlarge") {
			return 140.0
		}
		return 70.0 // m5.large approx.
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

	// Default.
	return 50.0
}
