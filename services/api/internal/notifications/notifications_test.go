package notifications

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type recordingQueue struct {
	bodies []string
}

func (queue *recordingQueue) Send(_ context.Context, body []byte) error {
	queue.bodies = append(queue.bodies, string(body))
	return nil
}

func TestRequestSendsTheWorkerContract(t *testing.T) {
	queue := &recordingQueue{}
	service := NewService(queue)
	service.now = func() time.Time { return time.Date(2026, 10, 27, 9, 30, 0, 0, time.UTC) }

	if err := service.Request(context.Background(), "  hello worker  "); err != nil {
		t.Fatal(err)
	}

	want := `{"type":"notification.requested","message":"hello worker","createdAt":"2026-10-27T09:30:00Z"}`
	if len(queue.bodies) != 1 || queue.bodies[0] != want {
		t.Fatalf("sent %v, want [%s]", queue.bodies, want)
	}
}

func TestRequestRejectsInvalidMessages(t *testing.T) {
	queue := &recordingQueue{}
	service := NewService(queue)

	for _, message := range []string{"", " ", strings.Repeat("x", MaxMessageLength+1)} {
		if err := service.Request(context.Background(), message); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("Request(%q) error = %v, want ErrInvalidMessage", message, err)
		}
	}
	if len(queue.bodies) != 0 {
		t.Fatal("invalid messages must not be sent")
	}
}
