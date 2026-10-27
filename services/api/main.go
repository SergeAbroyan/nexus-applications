package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"

	"github.com/SergeAbroyan/nexus-applications/services/api/internal/config"
	"github.com/SergeAbroyan/nexus-applications/services/api/internal/items"
	"github.com/SergeAbroyan/nexus-applications/services/api/internal/notifications"
	"github.com/SergeAbroyan/nexus-applications/services/api/internal/postgres"
	"github.com/SergeAbroyan/nexus-applications/services/api/internal/server"
	"github.com/SergeAbroyan/nexus-applications/services/api/internal/sqs"
	"github.com/SergeAbroyan/nexus-applications/services/api/internal/telemetry"
	"github.com/SergeAbroyan/nexus-applications/services/api/internal/valkey"
)

const serviceName = "nexus-api"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		if err := healthcheck(); err != nil {
			os.Exit(1)
		}
		return
	}

	logger := telemetry.NewLogger()
	if err := run(logger); err != nil {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	startup, cancelStartup := context.WithTimeout(ctx, 30*time.Second)
	defer cancelStartup()

	shutdownTracing, err := telemetry.SetupTracing(startup, serviceName)
	if err != nil {
		return fmt.Errorf("set up tracing: %w", err)
	}
	defer func() {
		flush, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(flush); err != nil {
			logger.Warn("flush traces", "error", err)
		}
	}()

	awsConfig, err := awsconfig.LoadDefaultConfig(startup)
	if err != nil {
		return fmt.Errorf("load AWS configuration: %w", err)
	}
	if awsConfig.Region == "" {
		return errors.New("AWS_REGION is required")
	}

	store, err := postgres.Open(startup, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	defer store.Close()

	cache, err := valkey.Connect(startup, cfg.Valkey, awsConfig)
	if err != nil {
		return fmt.Errorf("connect to Valkey: %w", err)
	}
	defer func() {
		if err := cache.Close(); err != nil {
			logger.Warn("close Valkey connection", "error", err)
		}
	}()

	handler := server.New(server.Dependencies{
		Logger:        logger,
		Metrics:       telemetry.NewMetrics("api"),
		Items:         items.NewService(store, cache, cfg.Valkey.CacheTTL, logger),
		Notifications: notifications.NewService(sqs.NewQueue(awsConfig, cfg.QueueURL)),
		Readiness:     []server.Check{{Name: "database", Ping: store.Ping}},
	})

	return serve(ctx, logger, &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	})
}

func serve(ctx context.Context, logger *slog.Logger, httpServer *http.Server) error {
	listenErrors := make(chan error, 1)
	go func() {
		logger.Info("api listening", "address", httpServer.Addr)
		listenErrors <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-listenErrors:
		return err
	case <-ctx.Done():
	}

	logger.Info("api shutting down")
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpServer.Shutdown(shutdown)
}

func healthcheck() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = config.DefaultPort
	}

	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://" + net.JoinHostPort("localhost", port) + "/healthz")
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned %d", response.StatusCode)
	}
	return nil
}
