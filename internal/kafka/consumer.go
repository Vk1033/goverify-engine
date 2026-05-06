package kafka

import (
	"github.com/segmentio/kafka-go"
	"github.com/vk1033/goverify-engine/internal/config"
)

type Consumers struct {
	EnrollReader *kafka.Reader
	VerifyReader *kafka.Reader
}

func NewConsumers(cfg *config.Config) *Consumers {
	er := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     cfg.Kafka.Brokers,
		GroupID:     "kyc-enroll-group-v7",
		Topic:       cfg.Kafka.EnrollTopic,
		StartOffset: kafka.FirstOffset,
		MinBytes:    1, // Process immediately
		MaxBytes:    10e6, // 10MB
	})

	vr := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     cfg.Kafka.Brokers,
		GroupID:     "kyc-verify-group-v7",
		Topic:       cfg.Kafka.VerifyTopic,
		StartOffset: kafka.FirstOffset,
		MinBytes:    1, // Process immediately
		MaxBytes:    10e6,
	})

	return &Consumers{
		EnrollReader: er,
		VerifyReader: vr,
	}
}

func (c *Consumers) Close() error {
	if err := c.EnrollReader.Close(); err != nil {
		return err
	}
	return c.VerifyReader.Close()
}
