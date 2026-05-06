//go:build noonnx

package embedding

import (
	"fmt"
	"github.com/rs/zerolog"
)

type DisabledService struct{}

func (s *DisabledService) GenerateFaceEmbedding(photoBase64 string) ([]float32, error) {
	return nil, fmt.Errorf("face embedding is disabled on this node (build tag: noonnx)")
}

func (s *DisabledService) GenerateNameEmbedding(name string) ([]float32, error) {
	return nil, fmt.Errorf("name embedding is disabled on this node (build tag: noonnx)")
}

func ProvideService(logger *zerolog.Logger) (Service, error) {
	logger.Warn().Msg("ONNX support disabled by build tag - embeddings will not be available locally")
	return &DisabledService{}, nil
}
