package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"9itchi-chat-api/models"
	"9itchi-chat-api/models/api"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeConvSvc struct {
	listQuery api.ListQuery
	startRole string
	startReq  api.StartConversationRequest
	err       *models.ApiError
}

func (f *fakeConvSvc) ListConversations(_ context.Context, _ string, q api.ListQuery) (*api.ConversationListResponse, *models.ApiError) {
	f.listQuery = q
	if f.err != nil {
		return nil, f.err
	}
	return &api.ConversationListResponse{Items: []api.ConversationResponse{}, Page: q.Page, PageSize: q.PageSize}, nil
}

func (f *fakeConvSvc) StartConversation(_ context.Context, _ string, role string, req api.StartConversationRequest) (*api.ConversationResponse, *models.ApiError) {
	f.startRole, f.startReq = role, req
	if f.err != nil {
		return nil, f.err
	}
	return &api.ConversationResponse{ID: "c1"}, nil
}

func (f *fakeConvSvc) MarkConversationRead(_ context.Context, _ string, id string) (*api.ConversationReadResponse, *models.ApiError) {
	if f.err != nil {
		return nil, f.err
	}
	return &api.ConversationReadResponse{ConversationID: id}, nil
}

func (f *fakeConvSvc) GlobalUnreadCount(context.Context, string) (*api.UnreadCountResponse, *models.ApiError) {
	if f.err != nil {
		return nil, f.err
	}
	return &api.UnreadCountResponse{UnreadCount: 4}, nil
}

func (f *fakeConvSvc) AblyToken(context.Context, string) (any, *models.ApiError) {
	return nil, models.NotImplemented()
}

type fakeMsgSvc struct {
	listQuery api.ListMessagesQuery
	sendReq   api.SendMessageRequest
	err       *models.ApiError
}

func (f *fakeMsgSvc) ListMessages(_ context.Context, _ string, _ string, q api.ListMessagesQuery) (*api.MessageListResponse, *models.ApiError) {
	f.listQuery = q
	if f.err != nil {
		return nil, f.err
	}
	return &api.MessageListResponse{Items: []api.MessageResponse{}}, nil
}

func (f *fakeMsgSvc) SendMessage(_ context.Context, _ string, _ string, req api.SendMessageRequest) (*api.MessageResponse, *models.ApiError) {
	f.sendReq = req
	if f.err != nil {
		return nil, f.err
	}
	return &api.MessageResponse{ID: "m1", Content: req.Content}, nil
}

func do(h gin.HandlerFunc, method, path, route, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", "u1")
		c.Set("role", "admin")
	})
	r.Handle(method, route, h)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func TestListConversationsHandler(t *testing.T) {
	svc := &fakeConvSvc{}
	w := do(ListConversations(svc), http.MethodGet, "/conversations?page=2&page_size=5", "/conversations", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, api.ListQuery{Page: 2, PageSize: 5}, svc.listQuery)

	w = do(ListConversations(svc), http.MethodGet, "/conversations?page=abc", "/conversations", "")
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestStartConversationHandler(t *testing.T) {
	svc := &fakeConvSvc{}
	w := do(StartConversation(svc), http.MethodPost, "/admin/conversations", "/admin/conversations", `{"user_id":"3f2b8c1e-0000-4000-8000-000000000001"}`)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "admin", svc.startRole)
	assert.Equal(t, "3f2b8c1e-0000-4000-8000-000000000001", svc.startReq.UserID)

	for _, body := range []string{`{}`, `{"user_id":"nope"}`, `not json`} {
		w = do(StartConversation(svc), http.MethodPost, "/admin/conversations", "/admin/conversations", body)
		assert.Equal(t, http.StatusBadRequest, w.Code, body)
		assert.Contains(t, w.Body.String(), `"error"`)
	}

	svc.err = &models.ApiError{Code: http.StatusForbidden, Error: errors.New("forbidden")}
	w = do(StartConversation(svc), http.MethodPost, "/admin/conversations", "/admin/conversations", `{"user_id":"3f2b8c1e-0000-4000-8000-000000000001"}`)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestMarkConversationReadHandler(t *testing.T) {
	svc := &fakeConvSvc{}
	w := do(MarkConversationRead(svc), http.MethodPut, "/conversations/abc/read", "/conversations/:id/read", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"conversation_id":"abc","unread_count":0}`, w.Body.String())

	svc.err = &models.ApiError{Code: http.StatusNotFound, Error: errors.New("conversation not found")}
	w = do(MarkConversationRead(svc), http.MethodPut, "/conversations/abc/read", "/conversations/:id/read", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "conversation not found")
}

func TestGlobalUnreadCountHandler(t *testing.T) {
	w := do(GlobalUnreadCount(&fakeConvSvc{}), http.MethodGet, "/user/global-unread-count", "/user/global-unread-count", "")
	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `{"unread_count":4}`, w.Body.String())
}

func TestAblyTokenHandlerStays501(t *testing.T) {
	w := do(AblyToken(&fakeConvSvc{}), http.MethodGet, "/chat/ably-token", "/chat/ably-token", "")
	assert.Equal(t, http.StatusNotImplemented, w.Code)
}

func TestListMessagesHandler(t *testing.T) {
	svc := &fakeMsgSvc{}
	w := do(ListMessages(svc), http.MethodGet, "/conversations/c1/messages?page_size=50&before=2024-01-01T00:00:00Z", "/conversations/:id/messages", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, api.ListMessagesQuery{PageSize: 50, Before: "2024-01-01T00:00:00Z"}, svc.listQuery)

	svc.err = &models.ApiError{Code: http.StatusNotFound, Error: errors.New("conversation not found")}
	w = do(ListMessages(svc), http.MethodGet, "/conversations/c1/messages", "/conversations/:id/messages", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestSendMessageHandler(t *testing.T) {
	svc := &fakeMsgSvc{}
	w := do(SendMessage(svc), http.MethodPost, "/conversations/c1/messages", "/conversations/:id/messages", `{"content":"hi"}`)
	assert.Equal(t, http.StatusCreated, w.Code)
	assert.Equal(t, "hi", svc.sendReq.Content)

	w = do(SendMessage(svc), http.MethodPost, "/conversations/c1/messages", "/conversations/:id/messages", `{}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	svc.err = &models.ApiError{Code: http.StatusNotFound, Error: errors.New("conversation not found")}
	w = do(SendMessage(svc), http.MethodPost, "/conversations/c1/messages", "/conversations/:id/messages", `{"content":"hi"}`)
	assert.Equal(t, http.StatusNotFound, w.Code)
}
