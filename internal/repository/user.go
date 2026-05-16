package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/vk1033/goverify-engine/internal/domain"
)

type UserRepository interface {
	SaveUser(ctx context.Context, user *domain.User) error
	GetUser(ctx context.Context, username string) (*domain.User, error)
	SaveRefreshToken(ctx context.Context, username, token string, duration time.Duration) error
	GetRefreshToken(ctx context.Context, username string) (string, error)
	DeleteRefreshToken(ctx context.Context, username string) error
}

type redisUserRepo struct {
	client *redis.Client
}

func NewUserRepository(client *redis.Client) UserRepository {
	return &redisUserRepo{client: client}
}

func (r *redisUserRepo) SaveUser(ctx context.Context, user *domain.User) error {
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, fmt.Sprintf("user:%s", user.Username), data, 0).Err()
}

func (r *redisUserRepo) GetUser(ctx context.Context, username string) (*domain.User, error) {
	data, err := r.client.Get(ctx, fmt.Sprintf("user:%s", username)).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("user not found")
	} else if err != nil {
		return nil, err
	}

	var user domain.User
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *redisUserRepo) SaveRefreshToken(ctx context.Context, username, token string, duration time.Duration) error {
	return r.client.Set(ctx, fmt.Sprintf("refresh_token:%s", username), token, duration).Err()
}

func (r *redisUserRepo) GetRefreshToken(ctx context.Context, username string) (string, error) {
	return r.client.Get(ctx, fmt.Sprintf("refresh_token:%s", username)).Result()
}

func (r *redisUserRepo) DeleteRefreshToken(ctx context.Context, username string) error {
	return r.client.Del(ctx, fmt.Sprintf("refresh_token:%s", username)).Err()
}
