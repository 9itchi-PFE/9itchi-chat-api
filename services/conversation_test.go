package services

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"9itchi-chat-api/models/api"
	"9itchi-chat-api/models/db"
	"9itchi-chat-api/repositories"
	"9itchi-chat-api/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartConversation(t *testing.T) {
	newRepo := func() *mocks.ConversationRepo {
		return &mocks.ConversationRepo{
			GetOrCreateFn: func(_ context.Context, a, u string) (*db.Conversation, error) {
				return &db.Conversation{ID: convID, AdminID: a, UserID: u}, nil
			},
			GetForParticipantFn: func(_ context.Context, id, uid string) (*db.ConversationSummary, error) {
				return &db.ConversationSummary{ID: id, AdminID: adminID, UserID: userID, UnreadCount: 2}, nil
			},
		}
	}
	tests := []struct {
		name     string
		role     string
		caller   string
		req      api.StartConversationRequest
		wantCode int
	}{
		{"success", "admin", adminID, api.StartConversationRequest{UserID: userID}, 0},
		{"non admin forbidden", "user", userID, api.StartConversationRequest{UserID: otherID}, http.StatusForbidden},
		{"bad user id", "admin", adminID, api.StartConversationRequest{UserID: "x"}, http.StatusBadRequest},
		{"self", "admin", adminID, api.StartConversationRequest{UserID: adminID}, http.StatusBadRequest},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := NewConversationService(newRepo())
			resp, apiErr := svc.StartConversation(context.Background(), tc.caller, tc.role, tc.req)
			if tc.wantCode != 0 {
				require.NotNil(t, apiErr)
				assert.Equal(t, tc.wantCode, apiErr.Code)
				assert.Nil(t, resp)
				return
			}
			require.Nil(t, apiErr)
			assert.Equal(t, convID, resp.ID)
			assert.Equal(t, 2, resp.UnreadCount)
		})
	}

	t.Run("repo error is 500", func(t *testing.T) {
		repo := newRepo()
		repo.GetOrCreateFn = func(context.Context, string, string) (*db.Conversation, error) { return nil, errors.New("boom") }
		_, apiErr := NewConversationService(repo).StartConversation(context.Background(), adminID, "admin", api.StartConversationRequest{UserID: userID})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
	})
}

func TestListConversations(t *testing.T) {
	content, sender, at := "hello", userID, time.Unix(10, 0)
	repo := &mocks.ConversationRepo{}
	repo.ListForUserFn = func(_ context.Context, uid string, limit, offset int) ([]db.ConversationSummary, int, error) {
		assert.Equal(t, adminID, uid)
		assert.Equal(t, 100, limit)
		assert.Equal(t, 100, offset)
		return []db.ConversationSummary{
			{ID: "c1", UnreadCount: 3, LastMessageContent: &content, LastMessageSenderID: &sender, LastMessageAt: &at},
			{ID: "c2"},
		}, 7, nil
	}
	resp, apiErr := NewConversationService(repo).ListConversations(context.Background(), adminID, api.ListQuery{Page: 2, PageSize: 1000})

	require.Nil(t, apiErr)
	assert.Equal(t, 7, resp.Total)
	assert.Equal(t, 2, resp.Page)
	assert.Equal(t, 100, resp.PageSize)
	require.Len(t, resp.Items, 2)
	assert.Equal(t, 3, resp.Items[0].UnreadCount)
	require.NotNil(t, resp.Items[0].LastMessage)
	assert.Equal(t, "hello", resp.Items[0].LastMessage.Content)
	assert.Nil(t, resp.Items[1].LastMessage)

	repo.ListForUserFn = func(context.Context, string, int, int) ([]db.ConversationSummary, int, error) {
		return nil, 0, errors.New("boom")
	}
	_, apiErr = NewConversationService(repo).ListConversations(context.Background(), adminID, api.ListQuery{})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
}

func TestMarkConversationRead(t *testing.T) {
	repo := &mocks.ConversationRepo{
		MarkReadFn: func(_ context.Context, id, uid string) error {
			if id == convID && uid == userID {
				return nil
			}
			return repositories.ErrNotFound
		},
	}
	svc := NewConversationService(repo)

	resp, apiErr := svc.MarkConversationRead(context.Background(), userID, convID)
	require.Nil(t, apiErr)
	assert.Equal(t, 0, resp.UnreadCount)

	_, apiErr = svc.MarkConversationRead(context.Background(), otherID, convID)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)

	_, apiErr = svc.MarkConversationRead(context.Background(), userID, "nope")
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotFound, apiErr.Code)
}

func TestGlobalUnreadCount(t *testing.T) {
	repo := &mocks.ConversationRepo{GlobalUnreadCountFn: func(_ context.Context, uid string) (int, error) { return 5, nil }}
	resp, apiErr := NewConversationService(repo).GlobalUnreadCount(context.Background(), userID)
	require.Nil(t, apiErr)
	assert.Equal(t, 5, resp.UnreadCount)

	repo.GlobalUnreadCountFn = func(context.Context, string) (int, error) { return 0, errors.New("boom") }
	_, apiErr = NewConversationService(repo).GlobalUnreadCount(context.Background(), userID)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
}

func TestAblyTokenNotImplemented(t *testing.T) {
	_, apiErr := NewConversationService(&mocks.ConversationRepo{}).AblyToken(context.Background(), userID)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusNotImplemented, apiErr.Code)
}
