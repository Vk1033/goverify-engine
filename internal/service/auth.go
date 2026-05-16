package service

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"github.com/vk1033/goverify-engine/internal/auth"
	"github.com/vk1033/goverify-engine/internal/domain"
	"github.com/vk1033/goverify-engine/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type AuthService interface {
	Register(ctx context.Context, req domain.RegisterRequest) error
	Login(ctx context.Context, req domain.AuthRequest) (*domain.AuthResponse, error)
	Refresh(ctx context.Context, req domain.RefreshRequest) (*domain.AuthResponse, error)
	Logout(ctx context.Context, username string) error
}

type authServiceImpl struct {
	userRepo   repository.UserRepository
	jwtManager *auth.JWTManager
	logger     *zerolog.Logger
}

func NewAuthService(repo repository.UserRepository, jwt *auth.JWTManager, logger *zerolog.Logger) AuthService {
	return &authServiceImpl{
		userRepo:   repo,
		jwtManager: jwt,
		logger:     logger,
	}
}

func (s *authServiceImpl) Register(ctx context.Context, req domain.RegisterRequest) error {
	// Check if user exists
	_, err := s.userRepo.GetUser(ctx, req.Username)
	if err == nil {
		return fmt.Errorf("user already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	user := &domain.User{
		Username:     req.Username,
		PasswordHash: string(hashedPassword),
		CreatedAt:    time.Now(),
	}

	return s.userRepo.SaveUser(ctx, user)
}

func (s *authServiceImpl) Login(ctx context.Context, req domain.AuthRequest) (*domain.AuthResponse, error) {
	user, err := s.userRepo.GetUser(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	accessToken, err := s.jwtManager.GenerateAccessToken(user.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Save refresh token in Redis (valid for 7 days as configured in JWTManager)
	err = s.userRepo.SaveRefreshToken(ctx, user.Username, refreshToken, 7*24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("failed to save refresh token: %w", err)
	}

	return &domain.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    900, // 15 minutes
	}, nil
}

func (s *authServiceImpl) Refresh(ctx context.Context, req domain.RefreshRequest) (*domain.AuthResponse, error) {
	claims, err := s.jwtManager.Verify(req.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token: %w", err)
	}

	storedToken, err := s.userRepo.GetRefreshToken(ctx, claims.Username)
	if err != nil || storedToken != req.RefreshToken {
		return nil, fmt.Errorf("refresh token expired or revoked")
	}

	newAccessToken, err := s.jwtManager.GenerateAccessToken(claims.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	return &domain.AuthResponse{
		AccessToken:  newAccessToken,
		RefreshToken: req.RefreshToken, // Keep using the same refresh token or rotate? Rotating is better but simpler for now.
		TokenType:    "Bearer",
		ExpiresIn:    900,
	}, nil
}

func (s *authServiceImpl) Logout(ctx context.Context, username string) error {
	return s.userRepo.DeleteRefreshToken(ctx, username)
}
