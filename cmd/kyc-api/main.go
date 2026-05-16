package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/rs/zerolog"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/vk1033/goverify-engine/internal/api"
	"github.com/vk1033/goverify-engine/internal/auth"
	"github.com/vk1033/goverify-engine/internal/config"
	"github.com/vk1033/goverify-engine/internal/embedding"
	"github.com/vk1033/goverify-engine/internal/kafka"
	"github.com/vk1033/goverify-engine/internal/observability"
	"github.com/vk1033/goverify-engine/internal/repository"
	"github.com/vk1033/goverify-engine/internal/service"
	"github.com/vk1033/goverify-engine/internal/vectordb"
	"github.com/vk1033/goverify-engine/pkg/logger"
)

func NewHTTPServer(lc fx.Lifecycle, cfg *config.Config, router *gin.Engine, log *zerolog.Logger) *http.Server {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: router,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info().Int("port", cfg.Port).Msg("Starting HTTP server")
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Error().Err(err).Msg("Failed to start server")
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info().Msg("Stopping HTTP server")
			return srv.Shutdown(ctx)
		},
	})

	return srv
}

var rootCmd = &cobra.Command{
	Use:   "kyc-api",
	Short: "KYC API Gateway",
	Run: func(cmd *cobra.Command, args []string) {
		app := fx.New(
			fx.Provide(
				config.LoadConfig,
				logger.NewLogger,
				api.NewRedisClient,
				kafka.NewProducer,
				embedding.ProvideService,
				vectordb.NewMilvusClient,
				repository.NewUserRepository,
				service.NewKYCService,
				service.NewAuthService,
				auth.ProvideJWTManager,
				api.NewHandler,
				api.NewRouter,
				NewHTTPServer,
			),
			fx.WithLogger(func(log *zerolog.Logger) fxevent.Logger {
				return &fxevent.ConsoleLogger{W: log}
			}),
			fx.Invoke(func(lc fx.Lifecycle, log *zerolog.Logger) {
				shutdown, err := observability.InitTracer(context.Background(), "kyc-api")
				if err != nil {
					log.Error().Err(err).Msg("Failed to initialize tracer")
					return
				}
				lc.Append(fx.Hook{
					OnStop: func(ctx context.Context) error {
						shutdown()
						return nil
					},
				})
			}),
			fx.Invoke(func(*http.Server) {}), // Start the server
		)
		app.Run()
	},
}

// @title           Distributed KYC Engine API
// @version         1.0
// @description     AI-driven KYC system with multi-modal identity signatures.
// @host            localhost:8080
// @BasePath        /
// @schemes         http

// @securityDefinitions.apikey Bearer
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and the JWT token.

func main() {
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}
