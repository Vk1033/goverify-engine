package embedding

import "math"

// Service defines the interface for generating embeddings.
type Service interface {
	GenerateFaceEmbedding(photoBase64 string) ([]float32, error)
	GenerateNameEmbedding(name string) ([]float32, error)
}

// normalize normalizes the vector to unit length for cosine similarity
func normalize(v []float32) []float32 {
	var sum float32
	for _, val := range v {
		sum += val * val
	}
	norm := float32(math.Sqrt(float64(sum)))
	if norm == 0 {
		return v
	}
	for i := range v {
		v[i] /= norm
	}
	return v
}
