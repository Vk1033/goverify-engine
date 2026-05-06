//go:build noonnx

package embedding

import (
	"github.com/rs/zerolog"
)

func ProvideService(logger *zerolog.Logger) (Service, error) {
	logger.Info().Msg("ONNX support disabled by build tag, using MockService")
	return NewMockService(), nil
}
