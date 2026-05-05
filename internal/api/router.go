package api

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	sloggin "github.com/samber/slog-gin"
	"github.com/vk1033/goverify-engine/internal/config"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/vk1033/goverify-engine/internal/observability"
	_ "github.com/vk1033/goverify-engine/docs"
)

func NewRouter(cfg *config.Config, logger *slog.Logger, handler *Handler, jwtManager *JWTManager) *gin.Engine {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// Use slog for gin logging
	r.Use(sloggin.New(logger))
	r.Use(gin.Recovery())
	r.Use(MetricsMiddleware())

	// Metrics (Public)
	r.GET("/metrics", gin.WrapH(observability.MetricsHandler()))
	r.GET("/health", handler.Health)

	// Swagger (Public)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Auth (Public)
	r.POST("/auth/login", handler.Login)

	// Authenticated routes
	authenticated := r.Group("/")
	authenticated.Use(AuthMiddleware(jwtManager))
	{
		api := authenticated.Group("/kyc")
		{
			api.POST("/enroll", handler.Enroll)
			api.POST("/verify", handler.Verify)
			api.GET("/status/:transaction_id", handler.Status)
			api.GET("/search", handler.Search)
		}
	}

	return r
}
