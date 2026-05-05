package logger

import (
	"context"
	"log/slog"
	"os"

	"github.com/vk1033/goverify-engine/internal/config"
	"go.opentelemetry.io/otel/trace"
)

// TraceIDHandler is a middleware slog.Handler that adds trace_id to logs
type TraceIDHandler struct {
	slog.Handler
}

func (h *TraceIDHandler) Handle(ctx context.Context, r slog.Record) error {
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		r.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func NewLogger(cfg *config.Config) *slog.Logger {
	var handler slog.Handler
	
	if cfg.Environment == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	}

	// Wrap handler with TraceIDHandler
	handler = &TraceIDHandler{Handler: handler}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}
