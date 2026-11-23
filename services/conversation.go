package services

import (
	"context"
	"errors"
	"net/http"
	"regexp"

	"9itchi-chat-api/models"
	"9itchi-chat-api/models/api"
	"9itchi-chat-api/models/mapping"
	"9itchi-chat-api/repositories"
	"9itchi-chat-api/utils"

	"go.uber.org/zap"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
	roleAdmin       = "admin"
)

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// normalizePage applies defaults (page 1, size 20) and the page size cap (100).
func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	return page, pageSize
}

func apiError(code int, msg string) *models.ApiError {
	return &models.ApiError{Code: code, Error: errors.New(msg)}
}

// repoError maps repository errors to API errors; unexpected errors are logged and hidden.
func repoError(err error, notFoundMsg string) *models.ApiError {
	switch {
	case errors.Is(err, repositories.ErrNotFound):
		return apiError(http.StatusNotFound, notFoundMsg)
	case errors.Is(err, repositories.ErrConflict):
		return apiError(http.StatusConflict, "conflict")
	default:
		utils.Logger.Error("repository error", zap.Error(err))
		return apiError(http.StatusInternalServerError, "internal server error")
	}
}

// ConversationService holds all business logic for conversation. Handlers stay thin.
type ConversationService interface {
	// GET /conversations: conversations the caller takes part in, updated_at desc
	ListConversations(ctx context.Context, userID string, query api.ListQuery) (*api.ConversationListResponse, *models.ApiError)
	// POST /admin/conversations: admin starts a chat with any user (idempotent per admin+user pair)
	StartConversation(ctx context.Context, userID, role string, req api.StartConversationRequest) (*api.ConversationResponse, *models.ApiError)
	// PUT /conversations/:id/read: reset the caller's unread count
	MarkConversationRead(ctx context.Context, userID string, id string) (*api.ConversationReadResponse, *models.ApiError)
	// GET /user/global-unread-count: total unread across conversations
	GlobalUnreadCount(ctx context.Context, userID string) (*api.UnreadCountResponse, *models.ApiError)
	// GET /chat/ably-token: scoped Ably token request
	AblyToken(ctx context.Context, userID string) (any, *models.ApiError)
}

type conversationService struct {
	repo repositories.ConversationRepository
}

func NewConversationService(repo repositories.ConversationRepository) ConversationService {
	return &conversationService{repo: repo}
}

func (s *conversationService) ListConversations(ctx context.Context, userID string, query api.ListQuery) (*api.ConversationListResponse, *models.ApiError) {
	page, pageSize := normalizePage(query.Page, query.PageSize)
	items, total, err := s.repo.ListForUser(ctx, userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, repoError(err, "conversation not found")
	}
	resp := &api.ConversationListResponse{
		Items:    make([]api.ConversationResponse, 0, len(items)),
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
	for _, it := range items {
		resp.Items = append(resp.Items, mapping.ConversationSummaryToResponse(it))
	}
	return resp, nil
}

func (s *conversationService) StartConversation(ctx context.Context, userID, role string, req api.StartConversationRequest) (*api.ConversationResponse, *models.ApiError) {
	if role != roleAdmin {
		return nil, apiError(http.StatusForbidden, "forbidden")
	}
	if !uuidRe.MatchString(req.UserID) {
		return nil, apiError(http.StatusBadRequest, "invalid user_id")
	}
	if req.UserID == userID {
		return nil, apiError(http.StatusBadRequest, "cannot start a conversation with yourself")
	}
	conv, err := s.repo.GetOrCreate(ctx, userID, req.UserID)
	if err != nil {
		return nil, repoError(err, "conversation not found")
	}
	summary, err := s.repo.GetForParticipant(ctx, conv.ID, userID)
	if err != nil {
		return nil, repoError(err, "conversation not found")
	}
	resp := mapping.ConversationSummaryToResponse(*summary)
	return &resp, nil
}

func (s *conversationService) MarkConversationRead(ctx context.Context, userID string, id string) (*api.ConversationReadResponse, *models.ApiError) {
	if !uuidRe.MatchString(id) {
		return nil, apiError(http.StatusNotFound, "conversation not found")
	}
	if err := s.repo.MarkRead(ctx, id, userID); err != nil {
		return nil, repoError(err, "conversation not found")
	}
	return &api.ConversationReadResponse{ConversationID: id, UnreadCount: 0}, nil
}

func (s *conversationService) GlobalUnreadCount(ctx context.Context, userID string) (*api.UnreadCountResponse, *models.ApiError) {
	n, err := s.repo.GlobalUnreadCount(ctx, userID)
	if err != nil {
		return nil, repoError(err, "not found")
	}
	return &api.UnreadCountResponse{UnreadCount: n}, nil
}

func (s *conversationService) AblyToken(_ context.Context, _ string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}
