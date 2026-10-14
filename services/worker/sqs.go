package main

import (
	"context"

	"github.com/SergeAbroyan/nexus-applications/services/observability"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type queueMessage struct {
	Body          string
	ReceiptHandle string
	TraceContext  map[string]string
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
	ctx, span := sqsClientSpan(ctx, "receive")
	defer span.End()

	result, err := queue.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:              aws.String(queue.queueURL),
		MaxNumberOfMessages:   10,
		WaitTimeSeconds:       queue.waitTimeSeconds,
		VisibilityTimeout:     queue.visibilityTimeoutSeconds,
		MessageAttributeNames: []string{"traceparent", "tracestate", "baggage"},
	})
	if err != nil {
		observability.RecordSpanError(span, err)
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
			TraceContext:  sqsTraceContext(message.MessageAttributes),
		})
	}
	return messages, nil
}

func (queue *sqsQueue) Delete(ctx context.Context, receiptHandle string) error {
	ctx, span := sqsClientSpan(ctx, "delete")
	defer span.End()

	_, err := queue.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(queue.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	observability.RecordSpanError(span, err)
	return err
}

func sqsTraceContext(attributes map[string]types.MessageAttributeValue) map[string]string {
	contextValues := make(map[string]string)
	for name, value := range attributes {
		if value.StringValue != nil {
			contextValues[name] = *value.StringValue
		}
	}
	return contextValues
}

func sqsClientSpan(ctx context.Context, operation string) (context.Context, trace.Span) {
	return otel.Tracer("nexus-worker/sqs").Start(
		ctx,
		"sqs."+operation,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("messaging.system", "aws_sqs"),
			attribute.String("messaging.operation.name", operation),
		),
	)
}
