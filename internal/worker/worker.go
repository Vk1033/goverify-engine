package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog"
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
	logger    *zerolog.Logger
}

func NewWorker(c *kafka.Consumers, s service.KYCService, r *redis.Client, l *zerolog.Logger) *Worker {
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
		w.logger.Info().Int("port", 9090).Msg("Starting worker metrics server")
		if err := http.ListenAndServe(":9090", observability.MetricsHandler()); err != nil {
			w.logger.Error().Err(err).Msg("Worker metrics server failed")
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
			w.logger.Error().Err(err).Msg("EnrollReader failed")
			time.Sleep(1 * time.Second)
			continue
		}

		observability.KafkaConsumerLagMs.Observe(float64(time.Since(m.Time).Milliseconds()))

		txnID := string(m.Key)
		var req domain.KYCRequest
		if err := json.Unmarshal(m.Value, &req); err != nil {
			w.logger.Error().Ctx(ctx).Err(err).Msg("failed to unmarshal enroll req")
			if rerr := w.redis.Set(ctx, txnID, string(domain.StatusError), 24*time.Hour).Err(); rerr != nil {
				w.logger.Error().Err(rerr).Str("txnID", txnID).Msg("Failed to update status in redis")
			}
			continue
		}

		if err := w.svc.ProcessEnrollment(ctx, txnID, req); err != nil {
			w.logger.Error().Ctx(ctx).Err(err).Str("txnID", txnID).Msg("failed to process enrollment")
			observability.KycEnrollmentsTotal.WithLabelValues("error").Inc()
			if rerr := w.redis.Set(ctx, txnID, string(domain.StatusError), 24*time.Hour).Err(); rerr != nil {
				w.logger.Error().Err(rerr).Str("txnID", txnID).Msg("Failed to update status in redis")
			}
			continue
		}

		observability.KycEnrollmentsTotal.WithLabelValues("success").Inc()

		if err := w.redis.Set(ctx, txnID, string(domain.StatusSuccess), 24*time.Hour).Err(); err != nil {
			w.logger.Error().Err(err).Str("txnID", txnID).Msg("failed to update status in redis")
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
			w.logger.Error().Err(err).Msg("VerifyReader failed")
			time.Sleep(1 * time.Second)
			continue
		}

		observability.KafkaConsumerLagMs.Observe(float64(time.Since(m.Time).Milliseconds()))

		txnID := string(m.Key)
		var req domain.KYCRequest
		if err := json.Unmarshal(m.Value, &req); err != nil {
			w.logger.Error().Ctx(ctx).Err(err).Msg("failed to unmarshal verify req")
			if rerr := w.redis.Set(ctx, txnID, string(domain.StatusError), 24*time.Hour).Err(); rerr != nil {
				w.logger.Error().Err(rerr).Str("txnID", txnID).Msg("Failed to update status in redis")
			}
			continue
		}

		res, err := w.svc.ProcessVerification(ctx, txnID, req)
		if err != nil {
			w.logger.Error().Ctx(ctx).Err(err).Str("txnID", txnID).Msg("failed to process verification")
			observability.KycVerificationsTotal.WithLabelValues("error").Inc()
			if rerr := w.redis.Set(ctx, txnID, string(domain.StatusError), 24*time.Hour).Err(); rerr != nil {
				w.logger.Error().Err(rerr).Str("txnID", txnID).Msg("Failed to update status in redis")
			}
			continue
		}

		observability.KycVerificationsTotal.WithLabelValues(string(res.Status)).Inc()
		observability.KycSimilarityScore.Observe(float64(res.ConfidenceScore))

		b, _ := json.Marshal(res)
		if err := w.redis.Set(ctx, txnID, b, 24*time.Hour).Err(); err != nil {
			w.logger.Error().Err(err).Str("txnID", txnID).Msg("failed to update status in redis")
		}

		if req.CallbackURL != "" {
			w.sendCallback(ctx, req.CallbackURL, res)
		}
	}
}

func (w *Worker) sendCallback(ctx context.Context, url string, payload interface{}) {
	b, err := json.Marshal(payload)
	if err != nil {
		w.logger.Error().Ctx(ctx).Err(err).Msg("failed to marshal callback payload")
		return
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(b))
	if err != nil {
		w.logger.Error().Ctx(ctx).Err(err).Msg("failed to create callback request")
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		w.logger.Error().Ctx(ctx).Err(err).Str("url", url).Msg("failed to send callback")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		w.logger.Error().Ctx(ctx).Str("status", resp.Status).Str("url", url).Msg("callback returned error status")
	} else {
		w.logger.Info().Ctx(ctx).Str("url", url).Msg("callback sent successfully")
	}
}
