package logger

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/vk1033/goverify-engine/internal/config"
	"go.opentelemetry.io/otel/trace"
)

// TracingHook is a zerolog hook that adds trace_id to logs
type TracingHook struct{}

func (h TracingHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	ctx := e.GetCtx()
	if ctx == nil {
		return
	}
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		e.Str("trace_id", span.SpanContext().TraceID().String()).
		  Str("span_id", span.SpanContext().SpanID().String())
	}
}

func NewLogger(cfg *config.Config) *zerolog.Logger {
	var logger zerolog.Logger
	
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs

	if cfg.Environment == "production" {
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger().Level(zerolog.InfoLevel)
	} else {
		// Use console writer for non-production environments
		consoleWriter := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05.000"}
		logger = zerolog.New(consoleWriter).With().Timestamp().Logger().Level(zerolog.DebugLevel)
	}

	// Add TracingHook
	logger = logger.Hook(TracingHook{})

	return &logger
}
