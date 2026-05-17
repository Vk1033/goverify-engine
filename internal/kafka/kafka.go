package kafka

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"

	"github.com/segmentio/kafka-go"
	"github.com/vk1033/goverify-engine/internal/config"
	"github.com/vk1033/goverify-engine/internal/domain"
)

type Producer interface {
	PublishEnrollment(ctx context.Context, txnID string, req domain.KYCRequest) error
	PublishVerification(ctx context.Context, txnID string, req domain.KYCRequest) error
	Close() error
}

type producerImpl struct {
	enrollWriter *kafka.Writer
	verifyWriter *kafka.Writer
	logger       *zerolog.Logger
}

func NewProducer(cfg *config.Config, logger *zerolog.Logger) Producer {
	ew := &kafka.Writer{
		Addr:       kafka.TCP(cfg.Kafka.Brokers...),
		Topic:      cfg.Kafka.EnrollTopic,
		Balancer:   &kafka.LeastBytes{},
		BatchBytes: 10485760,
	}
	vw := &kafka.Writer{
		Addr:       kafka.TCP(cfg.Kafka.Brokers...),
		Topic:      cfg.Kafka.VerifyTopic,
		Balancer:   &kafka.LeastBytes{},
		BatchBytes: 10485760,
	}
	return &producerImpl{
		enrollWriter: ew,
		verifyWriter: vw,
		logger:       logger,
	}
}

func (p *producerImpl) PublishEnrollment(ctx context.Context, txnID string, req domain.KYCRequest) error {
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}

	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	var headers []kafka.Header
	for k, v := range carrier {
		headers = append(headers, kafka.Header{Key: k, Value: []byte(v)})
	}

	msg := kafka.Message{
		Key:     []byte(txnID),
		Value:   b,
		Headers: headers,
		Time:    time.Now(),
	}
	return p.enrollWriter.WriteMessages(ctx, msg)
}

func (p *producerImpl) PublishVerification(ctx context.Context, txnID string, req domain.KYCRequest) error {
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}

	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	var headers []kafka.Header
	for k, v := range carrier {
		headers = append(headers, kafka.Header{Key: k, Value: []byte(v)})
	}

	msg := kafka.Message{
		Key:     []byte(txnID),
		Value:   b,
		Headers: headers,
		Time:    time.Now(),
	}
	return p.verifyWriter.WriteMessages(ctx, msg)
}

func (p *producerImpl) Close() error {
	if err := p.enrollWriter.Close(); err != nil {
		return err
	}
	return p.verifyWriter.Close()
}
