package embedding

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/yalue/onnxruntime_go"
)

func ProvideService(logger *zerolog.Logger) (Service, error) {
	modelPath := os.Getenv("ONNX_MODEL_PATH")
	libPath := os.Getenv("ONNX_LIB_PATH")

	if modelPath == "" || libPath == "" {
		logger.Warn().Msg("ONNX_MODEL_PATH or ONNX_LIB_PATH not set, falling back to MockService")
		return NewMockService(), nil
	}

	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		logger.Warn().Str("path", modelPath).Msg("ONNX model file not found, falling back to MockService")
		return NewMockService(), nil
	}

	onnxruntime_go.SetSharedLibraryPath(libPath)
	
	svc, err := NewONNXService(modelPath)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to initialize ONNX service, falling back to MockService")
		return NewMockService(), nil
	}

	logger.Info().Str("model", modelPath).Msg("Initialized real ONNX embedding service")
	return svc, nil
}
