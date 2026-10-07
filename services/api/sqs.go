package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type jobQueue struct {
	client   *sqs.Client
	queueURL string
}

func newJobQueue(configuration config, awsConfiguration aws.Config) *jobQueue {
	client := sqs.NewFromConfig(awsConfiguration, func(options *sqs.Options) {
		if configuration.SQSEndpoint != "" {
			options.BaseEndpoint = aws.String(configuration.SQSEndpoint)
		}
	})

	return &jobQueue{
		client:   client,
		queueURL: configuration.SQSQueueURL,
	}
}

func (queue *jobQueue) Publish(ctx context.Context, message string) error {
	body, err := json.Marshal(struct {
		Type      string    `json:"type"`
		Message   string    `json:"message"`
		CreatedAt time.Time `json:"createdAt"`
	}{
		Type:      "notification.requested",
		Message:   message,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		return err
	}

	_, err = queue.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(queue.queueURL),
		MessageBody: aws.String(string(body)),
	})
	return err
}
