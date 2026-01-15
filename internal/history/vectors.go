package history

import (
	"math"
)

// Vector represents resource state.
type Vector []float64

// Pattern definitions.
var (
	// Uniform scaling pattern.
	PatternUniformScaling = Normalize(Vector{1.0, 1.0, 0.5, 0.1}) // Balanced growth.
	
	// Anomaly pattern.
	PatternAnomaly = Normalize(Vector{0.1, 0.0, 1.0, 1.0}) // Waste spike.
)

// Normalize scales vector to unit length.
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

// DotProduct calculates vector dot product.
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
// Calculate cosine similarity.
//
//
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

// ClassifyPattern identifies vector patterns.
//
func ClassifyPattern(v Vector) string {
	// Check safe patterns.
	if CosineSimilarity(v, PatternUniformScaling) > 0.8 {
		return "SAFE"
	}
	
	// Check anomaly patterns.
	if CosineSimilarity(v, PatternAnomaly) > 0.8 {
		return "ANOMALY"
	}
	
	return "UNKNOWN"
}
