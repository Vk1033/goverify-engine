package main

import (
	"context"

	"github.com/rs/zerolog"

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

func RunWorker(lc fx.Lifecycle, w *worker.Worker, log *zerolog.Logger) {
	ctx, cancel := context.WithCancel(context.Background())
	
	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			log.Info().Msg("Starting KYC Worker...")
			w.Start(ctx)
			return nil
		},
		OnStop: func(_ context.Context) error {
			log.Info().Msg("Stopping KYC Worker...")
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
				embedding.ProvideService,
				vectordb.NewMilvusClient,
				kafka.NewConsumers,
				service.NewKYCService,
				worker.NewWorker,
			),
			fx.WithLogger(func(log *zerolog.Logger) fxevent.Logger {
				return &fxevent.ConsoleLogger{W: log}
			}),
			fx.Invoke(func(lc fx.Lifecycle, log *zerolog.Logger) {
				shutdown, err := observability.InitTracer(context.Background(), "kyc-worker")
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
