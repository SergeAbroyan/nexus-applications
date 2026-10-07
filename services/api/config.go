package main

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"
)

type config struct {
	Address                 string
	DatabaseURL             string
	AWSRegion               string
	SQSQueueURL             string
	SQSEndpoint             string
	ValkeyAddress           string
	ValkeyUsername          string
	ValkeyCacheName         string
	ValkeyIAMAuthentication bool
	ValkeyTLS               bool
	ValkeyCacheTTL          time.Duration
}

func loadConfig() (config, error) {
	port := environment("PORT", "8080")
	databaseHost := environment("DB_HOST", "localhost")
	databasePort := environment("DB_PORT", "5432")
	databaseName := environment("DB_NAME", "nexus")
	databaseUser := environment("DB_USER", "nexus")
	databasePassword := os.Getenv("DB_PASSWORD")
	databaseSSLMode := environment("DB_SSLMODE", "disable")
	valkeyHost := environment("VALKEY_HOST", "localhost")
	valkeyPort := environment("VALKEY_PORT", "6379")
	if databasePassword == "" {
		return config{}, fmt.Errorf("DB_PASSWORD is required")
	}

	valkeyIAMAuthentication, err := strconv.ParseBool(environment("VALKEY_IAM_AUTH", "false"))
	if err != nil {
		return config{}, fmt.Errorf("parse VALKEY_IAM_AUTH: %w", err)
	}

	valkeyTLS, err := strconv.ParseBool(environment("VALKEY_TLS", "false"))
	if err != nil {
		return config{}, fmt.Errorf("parse VALKEY_TLS: %w", err)
	}

	cacheTTL, err := time.ParseDuration(environment("VALKEY_CACHE_TTL", "30s"))
	if err != nil || cacheTTL <= 0 {
		return config{}, fmt.Errorf("VALKEY_CACHE_TTL must be a positive duration")
	}

	databaseURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(databaseUser, databasePassword),
		Host:   net.JoinHostPort(databaseHost, databasePort),
		Path:   databaseName,
	}
	databaseQuery := databaseURL.Query()
	databaseQuery.Set("sslmode", databaseSSLMode)
	databaseURL.RawQuery = databaseQuery.Encode()

	result := config{
		Address:                 net.JoinHostPort("", port),
		DatabaseURL:             databaseURL.String(),
		AWSRegion:               environment("AWS_REGION", "us-east-1"),
		SQSQueueURL:             os.Getenv("SQS_QUEUE_URL"),
		SQSEndpoint:             os.Getenv("SQS_ENDPOINT"),
		ValkeyAddress:           net.JoinHostPort(valkeyHost, valkeyPort),
		ValkeyUsername:          os.Getenv("VALKEY_USERNAME"),
		ValkeyCacheName:         os.Getenv("VALKEY_CACHE_NAME"),
		ValkeyIAMAuthentication: valkeyIAMAuthentication,
		ValkeyTLS:               valkeyTLS,
		ValkeyCacheTTL:          cacheTTL,
	}

	if result.SQSQueueURL == "" {
		return config{}, fmt.Errorf("SQS_QUEUE_URL is required")
	}

	if result.ValkeyIAMAuthentication && (result.ValkeyUsername == "" || result.ValkeyCacheName == "") {
		return config{}, fmt.Errorf("VALKEY_USERNAME and VALKEY_CACHE_NAME are required when VALKEY_IAM_AUTH is true")
	}

	return result, nil
}

func environment(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
