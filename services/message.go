package services

import (
	"context"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"9itchi-chat-api/core"
	"9itchi-chat-api/models"
	"9itchi-chat-api/models/api"
	"9itchi-chat-api/models/db"
	"9itchi-chat-api/models/mapping"
	"9itchi-chat-api/repositories"
	"9itchi-chat-api/utils"

	"go.uber.org/zap"
)

const maxMessageLength = 4000

// MessageService holds all business logic for message. Handlers stay thin.
type MessageService interface {
	// GET /conversations/:id/messages: message history (participants only), newest first
	ListMessages(ctx context.Context, userID string, id string, query api.ListMessagesQuery) (*api.MessageListResponse, *models.ApiError)
	// POST /conversations/:id/messages: store, bump unread for the other participant, then publish
	SendMessage(ctx context.Context, userID string, id string, req api.SendMessageRequest) (*api.MessageResponse, *models.ApiError)
}

type messageService struct {
	tx            repositories.Transactor
	conversations repositories.ConversationRepository
	messages      repositories.MessageRepository
	realtime      core.Realtime
}

func NewMessageService(tx repositories.Transactor, conversations repositories.ConversationRepository, messages repositories.MessageRepository, realtime core.Realtime) MessageService {
	return &messageService{tx: tx, conversations: conversations, messages: messages, realtime: realtime}
}

// requireParticipant returns a 404 unless userID takes part in conversation id.
func (s *messageService) requireParticipant(ctx context.Context, userID, id string) *models.ApiError {
	if !uuidRe.MatchString(id) {
		return apiError(http.StatusNotFound, "conversation not found")
	}
	if _, err := s.conversations.GetForParticipant(ctx, id, userID); err != nil {
		return repoError(err, "conversation not found")
	}
	return nil
}

func (s *messageService) ListMessages(ctx context.Context, userID string, id string, query api.ListMessagesQuery) (*api.MessageListResponse, *models.ApiError) {
	if apiErr := s.requireParticipant(ctx, userID, id); apiErr != nil {
		return nil, apiErr
	}
	var before *time.Time
	if query.Before != "" {
		t, err := time.Parse(time.RFC3339Nano, query.Before)
		if err != nil {
			return nil, apiError(http.StatusBadRequest, "invalid before cursor")
		}
		before = &t
	}
	page, pageSize := normalizePage(query.Page, query.PageSize)
	items, total, err := s.messages.List(ctx, id, before, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, repoError(err, "conversation not found")
	}
	resp := &api.MessageListResponse{
		Items:    make([]api.MessageResponse, 0, len(items)),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
	for _, m := range items {
		resp.Items = append(resp.Items, mapping.MessageToResponse(m))
	}
	return resp, nil
}

func (s *messageService) SendMessage(ctx context.Context, userID string, id string, req api.SendMessageRequest) (*api.MessageResponse, *models.ApiError) {
	content := strings.TrimSpace(req.Content)
	if n := utf8.RuneCountInString(content); n < 1 || n > maxMessageLength {
		return nil, apiError(http.StatusBadRequest, "content must be between 1 and 4000 characters")
	}
	if apiErr := s.requireParticipant(ctx, userID, id); apiErr != nil {
		return nil, apiErr
	}

	var msg *db.Message
	err := s.tx.WithTx(ctx, func(ctx context.Context) error {
		var err error
		if msg, err = s.messages.Create(ctx, id, userID, content); err != nil {
			return err
		}
		if err = s.conversations.Touch(ctx, id); err != nil {
			return err
		}
		return s.conversations.IncrementUnreadForOthers(ctx, id, userID)
	})
	if err != nil {
		return nil, repoError(err, "conversation not found")
	}

	resp := mapping.MessageToResponse(*msg)
	// The message is committed: a publish failure must not fail the request.
	if err := s.realtime.Publish(ctx, id, resp); err != nil {
		utils.Logger.Warn("realtime publish failed", zap.String("conversation_id", id), zap.Error(err))
	}
	return &resp, nil
}
