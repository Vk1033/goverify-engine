package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/vk1033/goverify-engine/internal/api"
	"github.com/vk1033/goverify-engine/internal/config"
	"github.com/vk1033/goverify-engine/internal/kafka"
	"github.com/vk1033/goverify-engine/pkg/logger"
)

func NewHTTPServer(lc fx.Lifecycle, cfg *config.Config, router *gin.Engine, log *slog.Logger) *http.Server {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Port),
		Handler: router,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			log.Info("Starting HTTP server", "port", cfg.Port)
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					log.Error("Failed to start server", "error", err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			log.Info("Stopping HTTP server")
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
				api.NewHandler,
				api.NewRouter,
				NewHTTPServer,
			),
			fx.WithLogger(func(log *slog.Logger) fxevent.Logger {
				return &fxevent.SlogLogger{Logger: log}
			}),
			fx.Invoke(func(*http.Server) {}), // Start the server
		)
		app.Run()
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}
