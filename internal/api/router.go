package api

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"
	"github.com/vk1033/goverify-engine/internal/config"
)

func NewRouter(cfg *config.Config, logger *slog.Logger, handler *Handler) *gin.Engine {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// Use slog for gin logging
	r.Use(sloggin.New(logger))
	r.Use(gin.Recovery())

	// Simple Auth Middleware
	r.Use(AuthMiddleware(cfg.JWT.Secret))

	api := r.Group("/kyc")
	{
		api.POST("/enroll", handler.Enroll)
		api.POST("/verify", handler.Verify)
		api.GET("/status/:transaction_id", handler.Status)
		api.GET("/search", handler.Search)
	}

	return r
}
