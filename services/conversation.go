package services

import (
	"context"

	"9itchi-chat-api/models"
	"9itchi-chat-api/models/api"
	"9itchi-chat-api/repositories"
)

// ConversationService holds all business logic for conversation. Handlers stay thin.
type ConversationService interface {
	// GET /conversations: own conversations (admin: all)
	ListConversations(ctx context.Context, userID string) (any, *models.ApiError)
	// POST /admin/conversations: admin starts a chat with any user
	StartConversation(ctx context.Context, userID string, req api.StartConversationRequest) (any, *models.ApiError)
	// PUT /conversations/:id/read: reset unread count
	MarkConversationRead(ctx context.Context, userID string, id string) (any, *models.ApiError)
	// GET /user/global-unread-count: total unread across conversations
	GlobalUnreadCount(ctx context.Context, userID string) (any, *models.ApiError)
	// GET /chat/ably-token: scoped Ably token request
	AblyToken(ctx context.Context, userID string) (any, *models.ApiError)
}

type conversationService struct {
	repo repositories.ConversationRepository
}

func NewConversationService(repo repositories.ConversationRepository) ConversationService {
	return &conversationService{repo: repo}
}

func (s *conversationService) ListConversations(_ context.Context, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *conversationService) StartConversation(_ context.Context, _ string, _ api.StartConversationRequest) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *conversationService) MarkConversationRead(_ context.Context, _ string, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *conversationService) GlobalUnreadCount(_ context.Context, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

func (s *conversationService) AblyToken(_ context.Context, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}
