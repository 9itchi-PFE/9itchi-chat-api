package core

import (
	"context"

	"9itchi-chat-api/utils"

	"go.uber.org/zap"
)

// Event names exchanged over RabbitMQ (exchange "9itchi.events"). Never rename a published name.
const (
	// EventSupportReplied: the team answered a student who has not read it yet (once per unread streak);
	// payload {user_id, conversation_id, unread}, never the text of a message. notification-api emails it.
	EventSupportReplied = "support.replied"
)

// Publisher sends domain events.
type Publisher interface {
	Publish(ctx context.Context, event string, payload any) error
}

// LogPublisher logs the event name only (never the payload). Used when RABBITMQ_URL is not set.
type LogPublisher struct{}

func (LogPublisher) Publish(_ context.Context, event string, _ any) error {
	utils.Logger.Info("event", zap.String("name", event))
	return nil
}
