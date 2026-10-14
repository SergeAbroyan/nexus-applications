package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"time"

	"github.com/SergeAbroyan/nexus-applications/services/observability"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type notification struct {
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type worker struct {
	logger *slog.Logger
	queue  jobQueue
	ready  atomic.Bool
}

func (worker *worker) Run(ctx context.Context) {
	worker.ready.Store(true)
	defer worker.ready.Store(false)

	for {
		if err := worker.runOnce(ctx); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return
			}

			worker.logger.WarnContext(ctx, "receive SQS messages", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}
}

func (worker *worker) runOnce(ctx context.Context) error {
	messages, err := worker.queue.Receive(ctx)
	if err != nil {
		return err
	}

	for _, message := range messages {
		worker.processMessage(ctx, message)
	}

	return nil
}

func (worker *worker) processMessage(ctx context.Context, message queueMessage) {
	ctx = observability.ExtractTraceContext(ctx, message.TraceContext)
	ctx, span := otel.Tracer("nexus-worker").Start(
		ctx,
		"notification.process",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "aws_sqs"),
			attribute.String("messaging.operation.name", "process"),
		),
	)
	defer span.End()

	notification, err := parseNotification(message.Body)
	if err != nil {
		observability.RecordSpanError(span, err)
		worker.logger.ErrorContext(ctx, "reject invalid notification", "error", err)
		return
	}

	worker.logger.InfoContext(
		ctx,
		"notification processed",
		"type", notification.Type,
		"message", notification.Message,
		"createdAt", notification.CreatedAt,
	)

	if err := worker.queue.Delete(ctx, message.ReceiptHandle); err != nil {
		observability.RecordSpanError(span, err)
		worker.logger.ErrorContext(ctx, "delete processed notification", "error", err)
	}
}

func parseNotification(body string) (notification, error) {
	var value notification
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return notification{}, fmt.Errorf("decode notification: %w", err)
	}

	if value.Type != "notification.requested" {
		return notification{}, fmt.Errorf("unsupported notification type %q", value.Type)
	}
	value.Message = strings.TrimSpace(value.Message)
	if value.Message == "" {
		return notification{}, fmt.Errorf("notification message is empty")
	}
	if value.CreatedAt.IsZero() {
		return notification{}, fmt.Errorf("notification timestamp is missing")
	}

	return value, nil
}
