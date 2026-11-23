// Package mocks holds hand-written test doubles for repository and core interfaces.
package mocks

import (
	"context"
	"time"

	"9itchi-chat-api/models/db"
)

// ConversationRepo is a function-field mock of repositories.ConversationRepository.
type ConversationRepo struct {
	GetOrCreateFn              func(ctx context.Context, adminID, userID string) (*db.Conversation, error)
	GetForParticipantFn        func(ctx context.Context, id, userID string) (*db.ConversationSummary, error)
	ListForUserFn              func(ctx context.Context, userID string, limit, offset int) ([]db.ConversationSummary, int, error)
	MarkReadFn                 func(ctx context.Context, id, userID string) error
	GlobalUnreadCountFn        func(ctx context.Context, userID string) (int, error)
	TouchFn                    func(ctx context.Context, id string) error
	IncrementUnreadForOthersFn func(ctx context.Context, id, senderID string) error
}

func (m *ConversationRepo) GetOrCreate(ctx context.Context, adminID, userID string) (*db.Conversation, error) {
	return m.GetOrCreateFn(ctx, adminID, userID)
}

func (m *ConversationRepo) GetForParticipant(ctx context.Context, id, userID string) (*db.ConversationSummary, error) {
	return m.GetForParticipantFn(ctx, id, userID)
}

func (m *ConversationRepo) ListForUser(ctx context.Context, userID string, limit, offset int) ([]db.ConversationSummary, int, error) {
	return m.ListForUserFn(ctx, userID, limit, offset)
}

func (m *ConversationRepo) MarkRead(ctx context.Context, id, userID string) error {
	return m.MarkReadFn(ctx, id, userID)
}

func (m *ConversationRepo) GlobalUnreadCount(ctx context.Context, userID string) (int, error) {
	return m.GlobalUnreadCountFn(ctx, userID)
}

func (m *ConversationRepo) Touch(ctx context.Context, id string) error { return m.TouchFn(ctx, id) }

func (m *ConversationRepo) IncrementUnreadForOthers(ctx context.Context, id, senderID string) error {
	return m.IncrementUnreadForOthersFn(ctx, id, senderID)
}

// MessageRepo is a function-field mock of repositories.MessageRepository.
type MessageRepo struct {
	CreateFn func(ctx context.Context, conversationID, senderID, content string) (*db.Message, error)
	ListFn   func(ctx context.Context, conversationID string, before *time.Time, limit, offset int) ([]db.Message, int, error)
}

func (m *MessageRepo) Create(ctx context.Context, conversationID, senderID, content string) (*db.Message, error) {
	return m.CreateFn(ctx, conversationID, senderID, content)
}

func (m *MessageRepo) List(ctx context.Context, conversationID string, before *time.Time, limit, offset int) ([]db.Message, int, error) {
	return m.ListFn(ctx, conversationID, before, limit, offset)
}

// Transactor runs fn directly and counts calls; Err is returned instead of running fn when set.
type Transactor struct {
	Calls int
	Err   error
}

func (m *Transactor) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	m.Calls++
	if m.Err != nil {
		return m.Err
	}
	return fn(ctx)
}

// Realtime records published events.
type Realtime struct {
	Channels []string
	Payloads []any
	Err      error
}

func (m *Realtime) Publish(_ context.Context, channel string, payload any) error {
	m.Channels = append(m.Channels, channel)
	m.Payloads = append(m.Payloads, payload)
	return m.Err
}
