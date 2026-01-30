package history

import (
	"math"
)

// Vector represents a multidimensional resource state.
type Vector []float64

// Predefined vector patterns.
var (
	// Pattern: Balanced growth.
	PatternUniformScaling = Normalize(Vector{1.0, 1.0, 0.5, 0.1}) 
	
	// Pattern: Anomalous waste spike.
	PatternAnomaly = Normalize(Vector{0.1, 0.0, 1.0, 1.0}) 
)

// Normalize scales the vector to unit length.
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

// CosineSimilarity calculates similarity.
// CosineSimilarity calculates the cosine similarity between vectors.
func CosineSimilarity(a, b Vector) float64 {
	// Calculate cosine similarity.
	//
	
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

// ClassifyPattern classifies the vector against known patterns.
func ClassifyPattern(v Vector) string {
	// Check for safe patterns.
	if CosineSimilarity(v, PatternUniformScaling) > 0.8 {
		return "SAFE"
	}
	
	// Check for anomaly patterns.
	if CosineSimilarity(v, PatternAnomaly) > 0.8 {
		return "ANOMALY"
	}
	
	return "UNKNOWN"
}
