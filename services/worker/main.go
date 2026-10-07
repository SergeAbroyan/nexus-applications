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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	configuration, err := loadConfig()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}

	startupContext, cancelStartup := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelStartup()
	awsConfiguration, err := awsconfig.LoadDefaultConfig(startupContext, awsconfig.WithRegion(configuration.AWSRegion))
	if err != nil {
		logger.Error("load AWS configuration", "error", err)
		os.Exit(1)
	}

	shutdownContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	worker := &worker{
		logger: logger,
		queue:  newSQSQueue(configuration, awsConfiguration),
	}
	go worker.Run(shutdownContext)

	server := &http.Server{
		Addr:              configuration.Address,
		Handler:           worker.handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("worker health server started", "address", server.Addr)
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			logger.Error("worker health server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-shutdownContext.Done():
		logger.Info("worker shutdown requested")
	}

	httpShutdownContext, cancelHTTPShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelHTTPShutdown()
	if err := server.Shutdown(httpShutdownContext); err != nil {
		logger.Error("worker graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}
