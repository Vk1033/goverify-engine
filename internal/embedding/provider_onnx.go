//go:build !noonnx

package embedding

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/yalue/onnxruntime_go"
)

func ProvideService(logger *zerolog.Logger) (Service, error) {
	modelPath := os.Getenv("ONNX_MODEL_PATH")
	libPath := os.Getenv("ONNX_LIB_PATH")

	if modelPath == "" || libPath == "" {
		return nil, fmt.Errorf("ONNX_MODEL_PATH or ONNX_LIB_PATH not set - AI service cannot start")
	}

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("ONNX model file not found at %s", modelPath)
	}

	onnxruntime_go.SetSharedLibraryPath(libPath)
	
	svc, err := NewONNXService(modelPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize real ONNX service: %w", err)
	}

	logger.Info().Str("model", modelPath).Msg("Initialized production ONNX embedding service")
	return svc, nil
}
