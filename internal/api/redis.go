package api

import (
	"context"

	"github.com/go-redis/redis/v8"
	"github.com/vk1033/goverify-engine/internal/config"
)

func NewRedisClient(cfg *config.Config) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: cfg.Redis.Address,
	})

	// Test connection
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}

	return client, nil
}
