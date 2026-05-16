package observability

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
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

	// New Metrics
	KycVerifyLatencyMs = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "goverify",
			Subsystem: "kyc",
			Name:      "verify_latency_ms",
			Help:      "Latency of KYC verification requests in milliseconds.",
			Buckets:   []float64{10, 50, 100, 250, 500, 1000, 2500, 5000},
		},
	)

	KafkaConsumerLagMs = promauto.NewSummary(
		prometheus.SummaryOpts{
			Namespace: "goverify",
			Subsystem: "kafka",
			Name:      "consumer_lag_ms",
			Help:      "Estimated consumer lag in milliseconds based on message processing delay.",
			Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
		},
	)

	VectorSearchLatencyMs = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "goverify",
			Subsystem: "vectordb",
			Name:      "search_latency_ms",
			Help:      "Latency of vector database similarity search in milliseconds.",
			Buckets:   []float64{5, 10, 25, 50, 100, 250, 500, 1000},
		},
	)
)

func InitTracer(ctx context.Context, serviceName string) (func(), error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		endpoint = "jaeger-svc:4317" // fallback to k8s service name
	}
	
	// Remove http:// prefix if present as OTLP gRPC exporter expects host:port
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")

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

