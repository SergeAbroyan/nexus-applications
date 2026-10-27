package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	RequestedType    = "notification.requested"
	MaxMessageLength = 500
)

var ErrInvalidMessage = fmt.Errorf("message must contain between 1 and %d characters", MaxMessageLength)

type Notification struct {
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"createdAt"`
}

type Queue interface {
	Send(ctx context.Context, body []byte) error
}

type Service struct {
	queue Queue
	now   func() time.Time
}

func NewService(queue Queue) *Service {
	return &Service{queue: queue, now: time.Now}
}

func (service *Service) Request(ctx context.Context, message string) error {
	message = strings.TrimSpace(message)
	if message == "" || utf8.RuneCountInString(message) > MaxMessageLength {
		return ErrInvalidMessage
	}

	body, err := json.Marshal(Notification{
		Type:      RequestedType,
		Message:   message,
		CreatedAt: service.now().UTC(),
	})
	if err != nil {
		return err
	}
	return service.queue.Send(ctx, body)
}
