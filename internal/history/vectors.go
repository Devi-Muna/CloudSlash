package history

import (
	"math"
)

// Vector represents a high-dimensional state of infrastructure resources.
type Vector []float64

// Known Patterns (Fingerprints)
var (
	// PatternUniformScaling represents a balanced growth across balanced resources (e.g. ASG scaling)
	PatternUniformScaling = Normalize(Vector{1.0, 1.0, 0.5, 0.1}) // High EC2/RDS, Low Waste
	
	// PatternAnomaly represents a waste-heavy spike (e.g. simple leak)
	PatternAnomaly = Normalize(Vector{0.1, 0.0, 1.0, 1.0}) // Low Compute, High Waste
)

// Normalize returns the unit vector.
func Normalize(v Vector) Vector {
	var sum float64
	for _, x := range v {
		sum += x * x
	}
	magnitude := math.Sqrt(sum)
	if magnitude == 0 {
		return v
	}

	result := make(Vector, len(v))
	for i, x := range v {
		result[i] = x / magnitude
	}
	return result
}

// DotProduct calculates the dot product of two vectors.
func DotProduct(a, b Vector) float64 {
	if len(a) != len(b) {
		return 0
	}
	var sum float64
	for i := range a {
		sum += a[i] * b[i]
	}
	return sum
}

// CosineSimilarity returns the cosine similarity between two vectors (-1 to 1).
// 1.0 = Identical direction
// 0.0 = Orthogonal (Unrelated)
// -1.0 = Opposite
func CosineSimilarity(a, b Vector) float64 {
	// Assumes a and b are NOT normalized, so we normalize them first or divide by magnitude.
	// For efficiency, if we maintain normalized vectors, we can just dot product.
	// Here we will compute safely.
	
	dot := DotProduct(a, b)
	
	var magA, magB float64
	for _, x := range a {
		magA += x * x
	}
	for _, x := range b {
		magB += x * x
	}
	
	magA = math.Sqrt(magA)
	magB = math.Sqrt(magB)
	
	if magA == 0 || magB == 0 {
		return 0
	}
	
	return dot / (magA * magB)
}

// ClassifyPattern determines if a transition vector looks like a known pattern.
// Returns "SAFE", "ANOMALY", or "UNKNOWN" based on similarity.
func ClassifyPattern(v Vector) string {
	// 1. Check against SAFE patterns
	if CosineSimilarity(v, PatternUniformScaling) > 0.8 {
		return "SAFE"
	}
	
	// 2. Check against ANOMALY patterns
	if CosineSimilarity(v, PatternAnomaly) > 0.8 {
		return "ANOMALY"
	}
	
	return "UNKNOWN"
}
