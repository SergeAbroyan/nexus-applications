package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/SergeAbroyan/nexus-applications/services/observability"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
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
	ctx, span := otel.Tracer("nexus-api/sqs").Start(
		ctx,
		"sqs.send",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(
			attribute.String("messaging.system", "aws_sqs"),
			attribute.String("messaging.operation.name", "send"),
		),
	)
	defer span.End()

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
		observability.RecordSpanError(span, err)
		return err
	}

	_, err = queue.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:          aws.String(queue.queueURL),
		MessageBody:       aws.String(string(body)),
		MessageAttributes: sqsTraceAttributes(ctx),
	})
	observability.RecordSpanError(span, err)
	return err
}

func sqsTraceAttributes(ctx context.Context) map[string]types.MessageAttributeValue {
	attributes := make(map[string]types.MessageAttributeValue)
	for name, value := range observability.InjectTraceContext(ctx) {
		attributes[name] = types.MessageAttributeValue{
			DataType:    aws.String("String"),
			StringValue: aws.String(value),
		}
	}
	return attributes
}
