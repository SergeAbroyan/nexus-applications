package main

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"
)

type fakeQueue struct {
	messages []queueMessage
	deleted  []string
}

func (queue *fakeQueue) Receive(context.Context) ([]queueMessage, error) {
	return queue.messages, nil
}

func (queue *fakeQueue) Delete(_ context.Context, receiptHandle string) error {
	queue.deleted = append(queue.deleted, receiptHandle)
	return nil
}

func TestProcessesAndDeletesValidNotification(t *testing.T) {
	queue := &fakeQueue{messages: []queueMessage{{
		Body:          `{"type":"notification.requested","message":"hello","createdAt":"2026-10-07T12:00:00Z"}`,
		ReceiptHandle: "receipt-1",
	}}}
	worker := &worker{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		queue:  queue,
	}

	if err := worker.runOnce(context.Background()); err != nil {
		t.Fatalf("process notification: %v", err)
	}
	if len(queue.deleted) != 1 || queue.deleted[0] != "receipt-1" {
		t.Fatal("expected the successfully processed notification to be deleted")
	}
}

func TestKeepsInvalidNotificationForRetry(t *testing.T) {
	queue := &fakeQueue{messages: []queueMessage{{
		Body:          `{"type":"unknown","message":"hello","createdAt":"2026-10-07T12:00:00Z"}`,
		ReceiptHandle: "receipt-2",
	}}}
	worker := &worker{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		queue:  queue,
	}

	if err := worker.runOnce(context.Background()); err != nil {
		t.Fatalf("process notification: %v", err)
	}
	if len(queue.deleted) != 0 {
		t.Fatal("expected the invalid notification to remain in SQS")
	}
}

func TestParsesNotification(t *testing.T) {
	createdAt := time.Date(2026, time.October, 7, 12, 0, 0, 0, time.UTC)
	value, err := parseNotification(`{"type":"notification.requested","message":" hello ","createdAt":"2026-10-07T12:00:00Z"}`)
	if err != nil {
		t.Fatalf("parse notification: %v", err)
	}
	if value.Message != "hello" || !value.CreatedAt.Equal(createdAt) {
		t.Fatal("expected normalized notification values")
	}
}
