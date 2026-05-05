package main

import (
	"context"
	"log/slog"

	"github.com/spf13/cobra"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"

	"github.com/vk1033/goverify-engine/internal/api"
	"github.com/vk1033/goverify-engine/internal/config"
	"github.com/vk1033/goverify-engine/internal/embedding"
	"github.com/vk1033/goverify-engine/internal/kafka"
	"github.com/vk1033/goverify-engine/internal/observability"
	"github.com/vk1033/goverify-engine/internal/service"
	"github.com/vk1033/goverify-engine/internal/vectordb"
	"github.com/vk1033/goverify-engine/internal/worker"
	"github.com/vk1033/goverify-engine/pkg/logger"
)

func RunWorker(lc fx.Lifecycle, w *worker.Worker, log *slog.Logger) {
	ctx, cancel := context.WithCancel(context.Background())
	
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			log.Info("Starting KYC Worker...")
			w.Start(ctx)
			return nil
		},
		OnStop: func(_ context.Context) error {
			log.Info("Stopping KYC Worker...")
			cancel()
			return nil
		},
	})
}

var rootCmd = &cobra.Command{
	Use:   "kyc-worker",
	Short: "KYC Async Processing Worker",
	Run: func(cmd *cobra.Command, args []string) {
		app := fx.New(
			fx.Provide(
				config.LoadConfig,
				logger.NewLogger,
				api.NewRedisClient,
				embedding.NewMockService,
				vectordb.NewMilvusClient,
				kafka.NewConsumers,
				service.NewKYCService,
				worker.NewWorker,
			),
			fx.WithLogger(func(log *slog.Logger) fxevent.Logger {
				return &fxevent.SlogLogger{Logger: log}
			}),
			fx.Invoke(func(lc fx.Lifecycle, log *slog.Logger) {
				shutdown, err := observability.InitTracer(context.Background(), "kyc-worker")
				if err != nil {
					log.Error("Failed to initialize tracer", "error", err)
					return
				}
				lc.Append(fx.Hook{
					OnStop: func(ctx context.Context) error {
						shutdown()
						return nil
					},
				})
			}),
			fx.Invoke(RunWorker), // Start the worker loops
		)
		app.Run()
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		panic(err)
	}
}
