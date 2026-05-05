package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/vk1033/goverify-engine/internal/domain"
	"github.com/vk1033/goverify-engine/internal/kafka"
	"github.com/vk1033/goverify-engine/internal/service"
)

type Worker struct {
	consumers *kafka.Consumers
	svc       service.KYCService
	redis     *redis.Client
	logger    *slog.Logger
}

func NewWorker(c *kafka.Consumers, s service.KYCService, r *redis.Client, l *slog.Logger) *Worker {
	return &Worker{
		consumers: c,
		svc:       s,
		redis:     r,
		logger:    l,
	}
}

func (w *Worker) Start(ctx context.Context) {
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
		var req domain.KYCEnrollRequest
		if err := json.Unmarshal(m.Value, &req); err != nil {
			w.logger.Error("failed to unmarshal enroll req", "error", err)
			if rerr := w.redis.Set(ctx, txnID, string(domain.StatusError), 24*time.Hour).Err(); rerr != nil {
				w.logger.Error("Failed to update status in redis", "error", rerr, "txnID", txnID)
			}
			continue
		}

		if err := w.svc.ProcessEnrollment(ctx, txnID, req); err != nil {
			w.logger.Error("failed to process enrollment", "error", err, "txnID", txnID)
			if rerr := w.redis.Set(ctx, txnID, string(domain.StatusError), 24*time.Hour).Err(); rerr != nil {
				w.logger.Error("Failed to update status in redis", "error", rerr, "txnID", txnID)
			}
			continue
		}

		if err := w.redis.Set(ctx, txnID, string(domain.StatusSuccess), 24*time.Hour).Err(); err != nil {
			w.logger.Error("failed to update status in redis", "error", err, "txnID", txnID)
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
		var req domain.KYCVerifyRequest
		if err := json.Unmarshal(m.Value, &req); err != nil {
			w.logger.Error("failed to unmarshal verify req", "error", err)
			if rerr := w.redis.Set(ctx, txnID, string(domain.StatusError), 24*time.Hour).Err(); rerr != nil {
				w.logger.Error("Failed to update status in redis", "error", rerr, "txnID", txnID)
			}
			continue
		}

		res, err := w.svc.ProcessVerification(ctx, txnID, req)
		if err != nil {
			w.logger.Error("failed to process verification", "error", err, "txnID", txnID)
			if rerr := w.redis.Set(ctx, txnID, string(domain.StatusError), 24*time.Hour).Err(); rerr != nil {
				w.logger.Error("Failed to update status in redis", "error", rerr, "txnID", txnID)
			}
			continue
		}

		b, _ := json.Marshal(res)
		if err := w.redis.Set(ctx, txnID, b, 24*time.Hour).Err(); err != nil {
			w.logger.Error("failed to update status in redis", "error", err, "txnID", txnID)
		}

		// In a real system, we'd also trigger a Callback URL if provided
	}
}
