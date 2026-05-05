package observability

import (
	"context"
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const (
	StatusHealth = "HEALTHY"
)

var (
	// HTTP Metrics
	// Dashboard: rate(goverify_http_requests_total[5m])
	// Alert: sum(rate(goverify_http_requests_total{status=~"5.."}[5m])) / sum(rate(goverify_http_requests_total[5m])) > 0.05
	HttpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "goverify",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests.",
		},
		[]string{"method", "path", "status"},
	)

	// Dashboard: histogram_quantile(0.99, sum(rate(goverify_http_request_duration_seconds_bucket[5m])) by (le, path))
	HttpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "goverify",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request duration in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)

	// KYC Metrics
	// Dashboard: rate(goverify_kyc_enrollments_total[5m])
	KycEnrollmentsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "goverify",
			Subsystem: "kyc",
			Name:      "enrollments_total",
			Help:      "Total number of KYC enrollments.",
		},
		[]string{"status"},
	)

	// Dashboard: rate(goverify_kyc_verifications_total[5m])
	KycVerificationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "goverify",
			Subsystem: "kyc",
			Name:      "verifications_total",
			Help:      "Total number of KYC verifications.",
		},
		[]string{"status"}, // matched, partial_match, no_match, error
	)

	// Dashboard: histogram_quantile(0.95, sum(rate(goverify_kyc_similarity_score_bucket[5m])) by (le))
	KycSimilarityScore = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "goverify",
			Subsystem: "kyc",
			Name:      "similarity_score",
			Help:      "Distribution of similarity scores for verification.",
			Buckets:   []float64{0, 0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
		},
	)
)

func InitTracer(ctx context.Context, serviceName string) (func(), error) {
	endpoint := "jaeger:4317"
	// For local testing outside docker, one might set this to localhost:4317
	// We'll leave it as jaeger:4317 by default for the docker-compose setup.

	exporter, err := otlptracegrpc.New(ctx, 
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(endpoint),
	)
	if err != nil {
		return nil, fmt.Errorf("creating OTLP exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("creating resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // Sample everything for hackathon
	)
	otel.SetTracerProvider(tp)

	shutdown := func() {
		_ = tp.Shutdown(context.Background())
	}
	return shutdown, nil
}

func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

func HealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "UP"}`))
	})
}
