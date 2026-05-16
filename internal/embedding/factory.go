package embedding

import (
	"fmt"
	"math"

	"github.com/rs/zerolog"
	"github.com/vk1033/goverify-engine/internal/config"
)

// Service defines the interface for generating embeddings.
type Service interface {
	GenerateIdentityEmbeddings(photoBase64 string, name string) (faceEmb []float32, nameEmb []float32, err error)
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

func ProvideService(cfg *config.Config, logger *zerolog.Logger) (Service, error) {
	if cfg.AIService.URL == "" {
		return nil, fmt.Errorf("AI_SERVICE_URL not set")
	}

	svc := NewPythonClient(cfg.AIService.URL, logger)
	logger.Info().Str("url", cfg.AIService.URL).Msg("Initialized Python-based AI service client")
	return svc, nil
}
