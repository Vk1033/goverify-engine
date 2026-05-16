package api

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/vk1033/goverify-engine/internal/config"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/vk1033/goverify-engine/internal/auth"
	"github.com/vk1033/goverify-engine/internal/observability"
	_ "github.com/vk1033/goverify-engine/docs"
)

func NewRouter(cfg *config.Config, logger *zerolog.Logger, handler *Handler, jwtManager *auth.JWTManager) *gin.Engine {
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()

	// Use custom zerolog middleware
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		logger.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Int("status", c.Writer.Status()).
			Dur("latency", time.Since(start)).
			Msg("HTTP Request")
	})
	r.Use(gin.Recovery())
	r.Use(MetricsMiddleware())

	// Metrics (Public)
	r.GET("/metrics", gin.WrapH(observability.MetricsHandler()))
	r.GET("/health", handler.Health)

	// Swagger (Public)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// Auth (Public)
	authGroup := r.Group("/auth")
	{
		authGroup.POST("/register", handler.Register)
		authGroup.POST("/login", handler.Login)
		authGroup.POST("/refresh", handler.Refresh)
	}

	// Authenticated routes
	authenticated := r.Group("/")
	authenticated.Use(AuthMiddleware(jwtManager))
	{
		authenticated.POST("/auth/logout", handler.Logout)

		kyc := authenticated.Group("/kyc")
		{
			kyc.POST("/enroll", handler.Enroll)
			kyc.POST("/verify", handler.Verify)
			kyc.GET("/status/:transaction_id", handler.Status)
			kyc.GET("/search", handler.Search)
		}
	}

	return r
}
