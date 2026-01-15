package aws

import (
	"strings"
)

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

// specsMap is a local cache of instance specs to avoid API calls for standard types.
// In a real production system, this should likely be synced from an API,
// but for v2.0 Enterprise, a robust static map is safer than a runtime dependency failure.
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
// Falls back to a generic estimation if unknown.
func GetSpecs(instanceType string) InstanceSpecs {
	if specs, ok := specsMap[instanceType]; ok {
		return specs
	}

	// Heuristic Fallback: Try to parse if unknown
	// e.g. "c5.9xlarge" -> guess based on family multiplier?
	// For now, return a safe "Large" default to strictly avoid 0/0 division errors.
	return InstanceSpecs{
		VCPU:   2,
		Memory: 8192,
		Arch:   "x86_64",
	}
}

// PricingStrategy defines the interface for determining costs.
type PricingStrategy interface {
	GetEstimatedCost(instanceType, region string) float64
}

// StaticCostEstimator provides a fallback if the API is unreachable.
// This replaces the "Ghost Node" 70.0 hardcoded value.
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
