package main

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type queueMessage struct {
	Body          string
	ReceiptHandle string
}

type jobQueue interface {
	Receive(context.Context) ([]queueMessage, error)
	Delete(context.Context, string) error
}

type sqsQueue struct {
	client                   *sqs.Client
	queueURL                 string
	waitTimeSeconds          int32
	visibilityTimeoutSeconds int32
}

func newSQSQueue(configuration config, awsConfiguration aws.Config) *sqsQueue {
	client := sqs.NewFromConfig(awsConfiguration, func(options *sqs.Options) {
		if configuration.SQSEndpoint != "" {
			options.BaseEndpoint = aws.String(configuration.SQSEndpoint)
		}
	})

	return &sqsQueue{
		client:                   client,
		queueURL:                 configuration.SQSQueueURL,
		waitTimeSeconds:          configuration.WaitTimeSeconds,
		visibilityTimeoutSeconds: configuration.VisibilityTimeoutSeconds,
	}
}

func (queue *sqsQueue) Receive(ctx context.Context) ([]queueMessage, error) {
	result, err := queue.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(queue.queueURL),
		MaxNumberOfMessages: 10,
		WaitTimeSeconds:     queue.waitTimeSeconds,
		VisibilityTimeout:   queue.visibilityTimeoutSeconds,
	})
	if err != nil {
		return nil, err
	}

	messages := make([]queueMessage, 0, len(result.Messages))
	for _, message := range result.Messages {
		if message.Body == nil || message.ReceiptHandle == nil {
			continue
		}
		messages = append(messages, queueMessage{
			Body:          *message.Body,
			ReceiptHandle: *message.ReceiptHandle,
		})
	}
	return messages, nil
}

func (queue *sqsQueue) Delete(ctx context.Context, receiptHandle string) error {
	_, err := queue.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(queue.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	return err
}
