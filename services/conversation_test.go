package services

import (
	"9itchi-chat-api/core"
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"9itchi-chat-api/models/api"
	"9itchi-chat-api/models/db"
	"9itchi-chat-api/repositories"
	"9itchi-chat-api/services/mocks"

	"github.com/ably/ably-go/ably"
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
			svc := NewConversationService(newRepo(), nil)
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
		_, apiErr := NewConversationService(repo, nil).StartConversation(context.Background(), adminID, "admin", api.StartConversationRequest{UserID: userID})
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
	resp, apiErr := NewConversationService(repo, nil).ListConversations(context.Background(), adminID, api.ListQuery{Page: 2, PageSize: 1000})

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
	_, apiErr = NewConversationService(repo, nil).ListConversations(context.Background(), adminID, api.ListQuery{})
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
	svc := NewConversationService(repo, nil)

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
	resp, apiErr := NewConversationService(repo, nil).GlobalUnreadCount(context.Background(), userID)
	require.Nil(t, apiErr)
	assert.Equal(t, 5, resp.UnreadCount)

	repo.GlobalUnreadCountFn = func(context.Context, string) (int, error) { return 0, errors.New("boom") }
	_, apiErr = NewConversationService(repo, nil).GlobalUnreadCount(context.Background(), userID)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
}

type fakeTokens struct {
	params *ably.TokenParams
	err    error
}

func (f *fakeTokens) CreateTokenRequest(params *ably.TokenParams, _ ...ably.AuthOption) (*ably.TokenRequest, error) {
	f.params = params
	if f.err != nil {
		return nil, f.err
	}
	return &ably.TokenRequest{TokenParams: *params, KeyName: "app.key"}, nil
}

func TestAblyToken(t *testing.T) {
	const allOps = `["subscribe","presence","history"]`
	listing := func(ids ...string) *mocks.ConversationRepo {
		return &mocks.ConversationRepo{ListForUserFn: func(_ context.Context, uid string, limit, offset int) ([]db.ConversationSummary, int, error) {
			assert.Equal(t, userID, uid)
			assert.Equal(t, maxTokenChannels, limit)
			assert.Equal(t, 0, offset)
			items := make([]db.ConversationSummary, 0, len(ids))
			for _, id := range ids {
				items = append(items, db.ConversationSummary{ID: id})
			}
			return items, len(items), nil
		}}
	}

	t.Run("grants only the caller conversations", func(t *testing.T) {
		tokens := &fakeTokens{}
		res, apiErr := NewConversationService(listing(convID, otherID), tokens).AblyToken(context.Background(), userID, userID)
		require.Nil(t, apiErr)
		assert.JSONEq(t, `{"`+convID+`":`+allOps+`,"`+otherID+`":`+allOps+`}`, res.Capability)
		assert.Equal(t, userID, tokens.params.ClientID)
		assert.Equal(t, (30 * time.Minute).Milliseconds(), tokens.params.TTL)
		assert.Equal(t, "app.key", res.KeyName)
	})

	t.Run("placeholder subscribe-only channel without conversations", func(t *testing.T) {
		res, apiErr := NewConversationService(listing(), &fakeTokens{}).AblyToken(context.Background(), userID, userID)
		require.Nil(t, apiErr)
		assert.JSONEq(t, `{"_no_conversations":["subscribe"]}`, res.Capability)
	})

	t.Run("unavailable without ably", func(t *testing.T) {
		_, apiErr := NewConversationService(listing(convID), nil).AblyToken(context.Background(), userID, userID)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusServiceUnavailable, apiErr.Code)
		assert.EqualError(t, apiErr.Error, "realtime unavailable")
	})

	t.Run("repo error is a generic 500", func(t *testing.T) {
		repo := &mocks.ConversationRepo{ListForUserFn: func(context.Context, string, int, int) ([]db.ConversationSummary, int, error) {
			return nil, 0, errors.New("boom")
		}}
		_, apiErr := NewConversationService(repo, &fakeTokens{}).AblyToken(context.Background(), userID, userID)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
	})

	t.Run("creator error is a generic 500", func(t *testing.T) {
		_, apiErr := NewConversationService(listing(convID), &fakeTokens{err: errors.New("key=secret")}).AblyToken(context.Background(), userID, userID)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
		assert.EqualError(t, apiErr.Error, "internal server error")
	})
}

// Support is a team inbox: whoever opens it, the conversation is the one between the team and the student.
func TestTeamInbox(t *testing.T) {
	var pairs [][2]string
	var seats []string
	repo := &mocks.ConversationRepo{
		GetOrCreateFn: func(_ context.Context, a, u string) (*db.Conversation, error) {
			pairs = append(pairs, [2]string{a, u})
			return &db.Conversation{ID: convID, AdminID: a, UserID: u}, nil
		},
		GetForParticipantFn: func(_ context.Context, id, seat string) (*db.ConversationSummary, error) {
			seats = append(seats, seat)
			return &db.ConversationSummary{ID: id, AdminID: core.TeamID, UserID: userID}, nil
		},
	}
	svc := NewConversationService(repo, nil)

	// A student opens it: seen from their own seat.
	resp, apiErr := svc.OpenSupport(context.Background(), userID, "candidate")
	require.Nil(t, apiErr)
	assert.Equal(t, convID, resp.ID)
	assert.Equal(t, core.TeamID, resp.AdminID)
	// Any admin writes to the student: the same pair, seen from the seat of the team.
	_, apiErr = svc.StartConversation(context.Background(), adminID, "admin", api.StartConversationRequest{UserID: userID})
	require.Nil(t, apiErr)
	_, apiErr = svc.StartConversation(context.Background(), otherID, "admin", api.StartConversationRequest{UserID: userID})
	require.Nil(t, apiErr)

	assert.Equal(t, [][2]string{{core.TeamID, userID}, {core.TeamID, userID}, {core.TeamID, userID}}, pairs)
	assert.Equal(t, []string{userID, core.TeamID, core.TeamID}, seats)

	// An admin has no conversation of their own, and nobody talks to the seat of the team itself.
	_, apiErr = svc.OpenSupport(context.Background(), adminID, "admin")
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusForbidden, apiErr.Code)
	_, apiErr = svc.OpenSupport(context.Background(), "nope", "candidate")
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	_, apiErr = svc.StartConversation(context.Background(), adminID, "admin", api.StartConversationRequest{UserID: core.TeamID})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)

	assert.Equal(t, core.TeamID, core.SeatOf(adminID, "admin"))
	assert.Equal(t, userID, core.SeatOf(userID, "candidate"))
}

func TestSetResolved(t *testing.T) {
	var got []bool
	at := time.Unix(50, 0)
	repo := &mocks.ConversationRepo{
		SetResolvedFn: func(_ context.Context, id string, resolved bool) error {
			if id != convID {
				return repositories.ErrNotFound
			}
			got = append(got, resolved)
			return nil
		},
		GetForParticipantFn: func(_ context.Context, id, seat string) (*db.ConversationSummary, error) {
			require.Equal(t, core.TeamID, seat, "the team's view of the conversation is returned")
			s := &db.ConversationSummary{ID: id, AdminID: core.TeamID, UserID: userID}
			if got[len(got)-1] {
				s.ResolvedAt = &at
			}
			return s, nil
		},
	}
	svc := NewConversationService(repo, nil)
	yes, no := true, false

	resp, apiErr := svc.SetResolved(context.Background(), convID, api.ResolveRequest{Resolved: &yes})
	require.Nil(t, apiErr)
	require.NotNil(t, resp.ResolvedAt)
	resp, apiErr = svc.SetResolved(context.Background(), convID, api.ResolveRequest{Resolved: &no})
	require.Nil(t, apiErr)
	assert.Nil(t, resp.ResolvedAt)
	assert.Equal(t, []bool{true, false}, got)

	_, apiErr = svc.SetResolved(context.Background(), convID, api.ResolveRequest{})
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	for _, id := range []string{"nope", otherID} {
		_, apiErr = svc.SetResolved(context.Background(), id, api.ResolveRequest{Resolved: &yes})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.Code)
	}
}
