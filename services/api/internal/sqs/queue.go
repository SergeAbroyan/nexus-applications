package sqs

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/aws/smithy-go/tracing/smithyoteltracing"
	"go.opentelemetry.io/otel"
)

type Queue struct {
	client *awssqs.Client
	url    string
}

func NewQueue(awsConfig aws.Config, queueURL string) *Queue {
	client := awssqs.NewFromConfig(awsConfig, func(options *awssqs.Options) {
		options.TracerProvider = smithyoteltracing.Adapt(otel.GetTracerProvider())
	})
	return &Queue{client: client, url: queueURL}
}

func (queue *Queue) Send(ctx context.Context, body []byte) error {
	attributes := messageAttributes{}
	otel.GetTextMapPropagator().Inject(ctx, attributes)

	_, err := queue.client.SendMessage(ctx, &awssqs.SendMessageInput{
		QueueUrl:          aws.String(queue.url),
		MessageBody:       aws.String(string(body)),
		MessageAttributes: attributes,
	})
	return err
}

type messageAttributes map[string]types.MessageAttributeValue

func (attributes messageAttributes) Get(key string) string {
	if value := attributes[key].StringValue; value != nil {
		return *value
	}
	return ""
}

func (attributes messageAttributes) Set(key, value string) {
	attributes[key] = types.MessageAttributeValue{DataType: aws.String("String"), StringValue: aws.String(value)}
}

func (attributes messageAttributes) Keys() []string {
	keys := make([]string, 0, len(attributes))
	for key := range attributes {
		keys = append(keys, key)
	}
	return keys
}
