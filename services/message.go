package services

import (
	"context"

	"9itchi-chat-api/models"
	"9itchi-chat-api/models/api"
	"9itchi-chat-api/repositories"
)

// MessageService holds all business logic for message. Handlers stay thin.
type MessageService interface {
	// GET /conversations/:id/messages: message history
	ListMessages(ctx context.Context, userID string, id string) (any, *models.ApiError)
	// POST /conversations/:id/messages: store then publish on Ably channel (conversationId)
	SendMessage(ctx context.Context, userID string, id string, req api.SendMessageRequest) (any, *models.ApiError)
}

type messageService struct {
	repo repositories.MessageRepository
}

func NewMessageService(repo repositories.MessageRepository) MessageService {
	return &messageService{repo: repo}
}

func (s *messageService) ListMessages(_ context.Context, _ string, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *messageService) SendMessage(_ context.Context, _ string, _ string, _ api.SendMessageRequest) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}
