package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		runHealthcheck()
		return
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	configuration, err := loadConfig()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}

	startupContext, cancelStartup := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelStartup()

	store, err := newPostgresStore(startupContext, configuration.DatabaseURL)
	if err != nil {
		logger.Error("connect to PostgreSQL", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	awsConfiguration, err := awsconfig.LoadDefaultConfig(startupContext, awsconfig.WithRegion(configuration.AWSRegion))
	if err != nil {
		logger.Error("load AWS configuration", "error", err)
		os.Exit(1)
	}

	cache := newValkeyCache(configuration, awsConfiguration.Credentials)
	defer func() {
		if err := cache.Close(); err != nil {
			logger.Warn("close Valkey connection", "error", err)
		}
	}()
	if err := cache.Ping(startupContext); err != nil {
		logger.Error("connect to Valkey", "error", err)
		os.Exit(1)
	}

	app := &application{
		logger: logger,
		store:  store,
		cache:  cache,
		queue:  newJobQueue(configuration, awsConfiguration),
	}
	server := &http.Server{
		Addr:              configuration.Address,
		Handler:           app.handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownSignal, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("api started", "address", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-shutdownSignal.Done():
		logger.Info("api shutdown requested")
	}

	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownContext); err != nil {
		logger.Error("api graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}

func runHealthcheck() {
	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://localhost:8080/healthz")
	if err != nil || response.StatusCode != http.StatusOK {
		os.Exit(1)
	}
	_ = response.Body.Close()
}
