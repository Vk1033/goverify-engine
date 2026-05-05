package worker

import (
	"context"
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/vk1033/goverify-engine/internal/domain"
	"github.com/vk1033/goverify-engine/internal/kafka"
	"github.com/vk1033/goverify-engine/internal/observability"
	"github.com/vk1033/goverify-engine/internal/service"
)

type Worker struct {
	consumers *kafka.Consumers
	svc       service.KYCService
	redis     *redis.Client
	client    *http.Client
	logger    *slog.Logger
}

func NewWorker(c *kafka.Consumers, s service.KYCService, r *redis.Client, l *slog.Logger) *Worker {
	return &Worker{
		consumers: c,
		svc:       s,
		redis:     r,
		client:    &http.Client{Timeout: 10 * time.Second},
		logger:    l,
	}
}

func (w *Worker) Start(ctx context.Context) {
	// Start Metrics Server for the worker
	go func() {
		w.logger.Info("Starting worker metrics server", "port", 9090)
		if err := http.ListenAndServe(":9090", observability.MetricsHandler()); err != nil {
			w.logger.Error("Worker metrics server failed", "error", err)
		}
	}()

	go w.consumeEnroll(ctx)
	go w.consumeVerify(ctx)
}

func (w *Worker) consumeEnroll(ctx context.Context) {
	for {
		m, err := w.consumers.EnrollReader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.logger.Error("EnrollReader failed", "error", err)
			time.Sleep(1 * time.Second)
			continue
		}

		txnID := string(m.Key)
		var req domain.KYCRequest
		if err := json.Unmarshal(m.Value, &req); err != nil {
			w.logger.ErrorContext(ctx, "failed to unmarshal enroll req", "error", err)
			if rerr := w.redis.Set(ctx, txnID, string(domain.StatusError), 24*time.Hour).Err(); rerr != nil {
				w.logger.Error("Failed to update status in redis", "error", rerr, "txnID", txnID)
			}
			continue
		}

		if err := w.svc.ProcessEnrollment(ctx, txnID, req); err != nil {
			w.logger.ErrorContext(ctx, "failed to process enrollment", "error", err, "txnID", txnID)
			observability.KycEnrollmentsTotal.WithLabelValues("error").Inc()
			if rerr := w.redis.Set(ctx, txnID, string(domain.StatusError), 24*time.Hour).Err(); rerr != nil {
				w.logger.Error("Failed to update status in redis", "error", rerr, "txnID", txnID)
			}
			continue
		}

		observability.KycEnrollmentsTotal.WithLabelValues("success").Inc()

		if err := w.redis.Set(ctx, txnID, string(domain.StatusSuccess), 24*time.Hour).Err(); err != nil {
			w.logger.Error("failed to update status in redis", "error", err, "txnID", txnID)
		}

		if req.CallbackURL != "" {
			w.sendCallback(ctx, req.CallbackURL, domain.VerificationResult{
				TransactionID: txnID,
				Status:        domain.StatusSuccess,
				CreatedAt:     time.Now(),
			})
		}
	}
}

func (w *Worker) consumeVerify(ctx context.Context) {
	for {
		m, err := w.consumers.VerifyReader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			w.logger.Error("VerifyReader failed", "error", err)
			time.Sleep(1 * time.Second)
			continue
		}

		txnID := string(m.Key)
		var req domain.KYCRequest
		if err := json.Unmarshal(m.Value, &req); err != nil {
			w.logger.ErrorContext(ctx, "failed to unmarshal verify req", "error", err)
			if rerr := w.redis.Set(ctx, txnID, string(domain.StatusError), 24*time.Hour).Err(); rerr != nil {
				w.logger.Error("Failed to update status in redis", "error", rerr, "txnID", txnID)
			}
			continue
		}

		res, err := w.svc.ProcessVerification(ctx, txnID, req)
		if err != nil {
			w.logger.ErrorContext(ctx, "failed to process verification", "error", err, "txnID", txnID)
			observability.KycVerificationsTotal.WithLabelValues("error").Inc()
			if rerr := w.redis.Set(ctx, txnID, string(domain.StatusError), 24*time.Hour).Err(); rerr != nil {
				w.logger.Error("Failed to update status in redis", "error", rerr, "txnID", txnID)
			}
			continue
		}

		observability.KycVerificationsTotal.WithLabelValues(string(res.Status)).Inc()
		observability.KycSimilarityScore.Observe(float64(res.ConfidenceScore))

		b, _ := json.Marshal(res)
		if err := w.redis.Set(ctx, txnID, b, 24*time.Hour).Err(); err != nil {
			w.logger.Error("failed to update status in redis", "error", err, "txnID", txnID)
		}

		if req.CallbackURL != "" {
			w.sendCallback(ctx, req.CallbackURL, res)
		}
	}
}

func (w *Worker) sendCallback(ctx context.Context, url string, payload interface{}) {
	b, err := json.Marshal(payload)
	if err != nil {
		w.logger.ErrorContext(ctx, "failed to marshal callback payload", "error", err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(b))
	if err != nil {
		w.logger.ErrorContext(ctx, "failed to create callback request", "error", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		w.logger.ErrorContext(ctx, "failed to send callback", "error", err, "url", url)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		w.logger.ErrorContext(ctx, "callback returned error status", "status", resp.Status, "url", url)
	} else {
		w.logger.InfoContext(ctx, "callback sent successfully", "url", url)
	}
}
