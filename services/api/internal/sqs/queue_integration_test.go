//go:build integration

package sqs

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestSendCarriesTheTraceContext(t *testing.T) {
	ctx := context.Background()

	container, err := testcontainers.Run(ctx, "softwaremill/elasticmq-native:1.7.1",
		testcontainers.WithExposedPorts("9324/tcp"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("9324/tcp")),
	)
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		t.Fatalf("start ElasticMQ: %v", err)
	}

	endpoint, err := container.PortEndpoint(ctx, "9324/tcp", "http")
	if err != nil {
		t.Fatalf("read ElasticMQ endpoint: %v", err)
	}

	awsConfig := aws.Config{
		Region:       "us-east-1",
		Credentials:  credentials.NewStaticCredentialsProvider("local", "local", ""),
		BaseEndpoint: aws.String(endpoint),
	}
	client := awssqs.NewFromConfig(awsConfig)
	created, err := client.CreateQueue(ctx, &awssqs.CreateQueueInput{QueueName: aws.String("nexus-jobs")})
	if err != nil {
		t.Fatalf("create queue: %v", err)
	}

	otel.SetTextMapPropagator(propagation.TraceContext{})
	provider := sdktrace.NewTracerProvider()
	defer provider.Shutdown(ctx)
	traced, span := provider.Tracer("test").Start(ctx, "request")
	defer span.End()

	if err := NewQueue(awsConfig, *created.QueueUrl).Send(traced, []byte(`{"message":"hello"}`)); err != nil {
		t.Fatalf("send: %v", err)
	}

	received, err := client.ReceiveMessage(ctx, &awssqs.ReceiveMessageInput{
		QueueUrl:              created.QueueUrl,
		MessageAttributeNames: []string{"All"},
		WaitTimeSeconds:       1,
	})
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if len(received.Messages) != 1 {
		t.Fatalf("received %d messages, want 1", len(received.Messages))
	}

	message := received.Messages[0]
	if *message.Body != `{"message":"hello"}` {
		t.Fatalf("body = %s", *message.Body)
	}
	traceParent := message.MessageAttributes["traceparent"].StringValue
	if traceParent == nil || len(*traceParent) != 55 || (*traceParent)[3:35] != span.SpanContext().TraceID().String() {
		t.Fatalf("traceparent = %v, want trace %s", traceParent, span.SpanContext().TraceID())
	}
}
