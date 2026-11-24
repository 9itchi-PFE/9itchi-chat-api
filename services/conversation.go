package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"time"

	"9itchi-chat-api/core"
	"9itchi-chat-api/models"
	"9itchi-chat-api/models/api"
	"9itchi-chat-api/models/mapping"
	"9itchi-chat-api/repositories"
	"9itchi-chat-api/utils"

	"github.com/ably/ably-go/ably"
	"go.uber.org/zap"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
	roleAdmin       = "admin"
	// ablyTokenTTL keeps tokens short so membership changes reach the capability quickly.
	ablyTokenTTL = 30 * time.Minute
	// maxTokenChannels bounds the capability size.
	maxTokenChannels = maxPageSize
	// noConversationsChannel is granted (subscribe only) to a user without conversations: an Ably capability cannot be empty.
	noConversationsChannel = "_no_conversations"
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
	// GET /conversations: the conversations of a seat (the team for an admin, the student for a student), updated_at desc
	ListConversations(ctx context.Context, userID string, query api.ListQuery) (*api.ConversationListResponse, *models.ApiError)
	// POST /conversations: the student's conversation with the team, created on first use
	OpenSupport(ctx context.Context, userID, role string) (*api.ConversationResponse, *models.ApiError)
	// POST /admin/conversations: the team's conversation with a student, created on first use
	StartConversation(ctx context.Context, userID, role string, req api.StartConversationRequest) (*api.ConversationResponse, *models.ApiError)
	// PUT /conversations/:id/read: reset the caller's unread count
	MarkConversationRead(ctx context.Context, userID string, id string) (*api.ConversationReadResponse, *models.ApiError)
	// GET /user/global-unread-count: total unread across conversations
	GlobalUnreadCount(ctx context.Context, userID string) (*api.UnreadCountResponse, *models.ApiError)
	// GET /chat/ably-token: Ably token request scoped to the conversation channels of the seat, for the client
	// userID (503 when Ably is not configured)
	AblyToken(ctx context.Context, seatID, userID string) (*api.AblyTokenResponse, *models.ApiError)
}

type conversationService struct {
	repo   repositories.ConversationRepository
	tokens core.TokenRequestCreator // nil when Ably is not configured
}

func NewConversationService(repo repositories.ConversationRepository, tokens core.TokenRequestCreator) ConversationService {
	return &conversationService{repo: repo, tokens: tokens}
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
	if req.UserID == userID || req.UserID == core.TeamID {
		return nil, apiError(http.StatusBadRequest, "cannot start a conversation with yourself")
	}
	return s.open(ctx, req.UserID, core.TeamID)
}

// OpenSupport gives a student their conversation with the team. An admin has no conversation of their own: the
// team writes to a student through StartConversation.
func (s *conversationService) OpenSupport(ctx context.Context, userID, role string) (*api.ConversationResponse, *models.ApiError) {
	if role == roleAdmin {
		return nil, apiError(http.StatusForbidden, "forbidden")
	}
	if !uuidRe.MatchString(userID) || userID == core.TeamID {
		return nil, apiError(http.StatusBadRequest, "invalid user id")
	}
	return s.open(ctx, userID, userID)
}

// open returns the one conversation between the team and a student, creating it if missing, as seen from a seat.
func (s *conversationService) open(ctx context.Context, studentID, seatID string) (*api.ConversationResponse, *models.ApiError) {
	conv, err := s.repo.GetOrCreate(ctx, core.TeamID, studentID)
	if err != nil {
		return nil, repoError(err, "conversation not found")
	}
	summary, err := s.repo.GetForParticipant(ctx, conv.ID, seatID)
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

func (s *conversationService) AblyToken(ctx context.Context, seatID, userID string) (*api.AblyTokenResponse, *models.ApiError) {
	if s.tokens == nil {
		return nil, apiError(http.StatusServiceUnavailable, "realtime unavailable")
	}
	// Channel name = conversation id; the most recently updated conversations are granted.
	items, _, err := s.repo.ListForUser(ctx, seatID, maxTokenChannels, 0)
	if err != nil {
		return nil, repoError(err, "conversation not found")
	}
	caps := make(map[string][]string, len(items))
	for _, it := range items {
		// No publish: messages only enter through SendMessage, so a client cannot push unpersisted or forged events.
		caps[it.ID] = []string{"subscribe", "presence", "history"}
	}
	if len(caps) == 0 {
		caps[noConversationsChannel] = []string{"subscribe"}
	}
	capability, err := json.Marshal(caps)
	if err != nil {
		utils.Logger.Error("build ably capability", zap.Error(err))
		return nil, apiError(http.StatusInternalServerError, "internal server error")
	}
	req, err := s.tokens.CreateTokenRequest(&ably.TokenParams{
		TTL:        ablyTokenTTL.Milliseconds(),
		Capability: string(capability),
		ClientID:   userID,
	})
	if err != nil {
		utils.Logger.Error("create ably token request", zap.Error(err))
		return nil, apiError(http.StatusInternalServerError, "internal server error")
	}
	return req, nil
}
