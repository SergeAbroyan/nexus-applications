package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

type config struct {
	Address                  string
	AWSRegion                string
	SQSQueueURL              string
	SQSEndpoint              string
	WaitTimeSeconds          int32
	VisibilityTimeoutSeconds int32
}

func loadConfig() (config, error) {
	waitTime, err := parseInt32(environment("SQS_WAIT_TIME_SECONDS", "20"))
	if err != nil || waitTime < 1 || waitTime > 20 {
		return config{}, fmt.Errorf("SQS_WAIT_TIME_SECONDS must be between 1 and 20")
	}

	visibilityTimeout, err := parseInt32(environment("SQS_VISIBILITY_TIMEOUT_SECONDS", "60"))
	if err != nil || visibilityTimeout < 1 || visibilityTimeout > 43200 {
		return config{}, fmt.Errorf("SQS_VISIBILITY_TIMEOUT_SECONDS must be between 1 and 43200")
	}

	result := config{
		Address:                  net.JoinHostPort("", environment("PORT", "8080")),
		AWSRegion:                environment("AWS_REGION", "us-east-1"),
		SQSQueueURL:              os.Getenv("SQS_QUEUE_URL"),
		SQSEndpoint:              os.Getenv("SQS_ENDPOINT"),
		WaitTimeSeconds:          waitTime,
		VisibilityTimeoutSeconds: visibilityTimeout,
	}

	if result.SQSQueueURL == "" {
		return config{}, fmt.Errorf("SQS_QUEUE_URL is required")
	}

	return result, nil
}

func environment(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func parseInt32(value string) (int32, error) {
	parsed, err := strconv.ParseInt(value, 10, 32)
	return int32(parsed), err
}
