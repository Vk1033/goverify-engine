package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/vk1033/goverify-engine/internal/config"
)

type JWTManager struct {
	secretKey       string
	accessDuration  time.Duration
	refreshDuration time.Duration
}

func NewJWTManager(secretKey string, accessDuration, refreshDuration time.Duration) *JWTManager {
	return &JWTManager{secretKey, accessDuration, refreshDuration}
}

type UserClaims struct {
	jwt.RegisteredClaims
	Username string `json:"username"`
}

func (manager *JWTManager) GenerateAccessToken(username string) (string, error) {
	claims := UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(manager.accessDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Username: username,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(manager.secretKey))
}

func (manager *JWTManager) GenerateRefreshToken(username string) (string, error) {
	claims := UserClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(manager.refreshDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Username: username,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(manager.secretKey))
}

func (manager *JWTManager) Verify(tokenString string) (*UserClaims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&UserClaims{},
		func(token *jwt.Token) (interface{}, error) {
			_, ok := token.Method.(*jwt.SigningMethodHMAC)
			if !ok {
				return nil, fmt.Errorf("unexpected token signing method")
			}

			return []byte(manager.secretKey), nil
		},
	)

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*UserClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}

func ProvideJWTManager(cfg *config.Config) *JWTManager {
	return NewJWTManager(cfg.JWT.Secret, 15*time.Minute, 7*24*time.Hour)
}
