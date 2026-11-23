package services

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"9itchi-chat-api/models/api"
	"9itchi-chat-api/models/db"
	"9itchi-chat-api/repositories"
	"9itchi-chat-api/services/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	convID  = "11111111-1111-1111-1111-111111111111"
	adminID = "22222222-2222-2222-2222-222222222222"
	userID  = "33333333-3333-3333-3333-333333333333"
	otherID = "44444444-4444-4444-4444-444444444444"
)

type msgFixture struct {
	svc   MessageService
	convs *mocks.ConversationRepo
	msgs  *mocks.MessageRepo
	tx    *mocks.Transactor
	rt    *mocks.Realtime
	calls []string
}

// newMsgFixture builds a service where only adminID and userID are participants of convID.
func newMsgFixture() *msgFixture {
	f := &msgFixture{tx: &mocks.Transactor{}, rt: &mocks.Realtime{}}
	f.convs = &mocks.ConversationRepo{
		GetForParticipantFn: func(_ context.Context, id, uid string) (*db.ConversationSummary, error) {
			if id == convID && (uid == adminID || uid == userID) {
				return &db.ConversationSummary{ID: id}, nil
			}
			return nil, repositories.ErrNotFound
		},
		TouchFn: func(_ context.Context, id string) error {
			f.calls = append(f.calls, "touch:"+id)
			return nil
		},
		IncrementUnreadForOthersFn: func(_ context.Context, id, sender string) error {
			f.calls = append(f.calls, "unread:"+id+":"+sender)
			return nil
		},
	}
	f.msgs = &mocks.MessageRepo{
		CreateFn: func(_ context.Context, cid, sender, content string) (*db.Message, error) {
			f.calls = append(f.calls, "create:"+content)
			return &db.Message{ID: "m1", ConversationID: cid, SenderID: sender, Content: content, CreatedAt: time.Unix(1, 0)}, nil
		},
	}
	f.svc = NewMessageService(f.tx, f.convs, f.msgs, f.rt)
	return f
}

func TestSendMessage_Success(t *testing.T) {
	f := newMsgFixture()

	resp, apiErr := f.svc.SendMessage(context.Background(), userID, convID, api.SendMessageRequest{Content: "  hello  "})

	require.Nil(t, apiErr)
	assert.Equal(t, "hello", resp.Content)
	assert.Equal(t, userID, resp.SenderID)
	// One transaction: insert, bump updated_at, increment the other participant's unread count.
	assert.Equal(t, 1, f.tx.Calls)
	assert.Equal(t, []string{"create:hello", "touch:" + convID, "unread:" + convID + ":" + userID}, f.calls)
	assert.Equal(t, []string{convID}, f.rt.Channels)
	assert.Equal(t, *resp, f.rt.Payloads[0])
}

func TestSendMessage_Validation(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"empty", ""},
		{"whitespace only", " \n\t "},
		{"too long", strings.Repeat("a", 4001)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newMsgFixture()
			resp, apiErr := f.svc.SendMessage(context.Background(), userID, convID, api.SendMessageRequest{Content: tc.content})
			assert.Nil(t, resp)
			require.NotNil(t, apiErr)
			assert.Equal(t, http.StatusBadRequest, apiErr.Code)
			assert.Empty(t, f.calls)
		})
	}
}

func TestSendMessage_MaxLengthAccepted(t *testing.T) {
	f := newMsgFixture()
	_, apiErr := f.svc.SendMessage(context.Background(), adminID, convID, api.SendMessageRequest{Content: strings.Repeat("é", 4000)})
	assert.Nil(t, apiErr)
}

func TestSendMessage_ParticipantIsolation(t *testing.T) {
	tests := []struct {
		name   string
		caller string
		convID string
	}{
		{"non participant", otherID, convID},
		{"unknown conversation", userID, "55555555-5555-5555-5555-555555555555"},
		{"malformed id", userID, "not-a-uuid"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := newMsgFixture()
			resp, apiErr := f.svc.SendMessage(context.Background(), tc.caller, tc.convID, api.SendMessageRequest{Content: "hi"})
			assert.Nil(t, resp)
			require.NotNil(t, apiErr)
			assert.Equal(t, http.StatusNotFound, apiErr.Code)
			assert.Empty(t, f.calls, "nothing may be written")
			assert.Zero(t, f.tx.Calls)
			assert.Empty(t, f.rt.Channels, "nothing may be published")
		})
	}
}

func TestSendMessage_RepoFailureRollsBackAndSkipsPublish(t *testing.T) {
	f := newMsgFixture()
	f.convs.IncrementUnreadForOthersFn = func(context.Context, string, string) error { return errors.New("db down: secret detail") }

	resp, apiErr := f.svc.SendMessage(context.Background(), userID, convID, api.SendMessageRequest{Content: "hi"})

	assert.Nil(t, resp)
	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
	assert.NotContains(t, apiErr.Error.Error(), "secret")
	assert.Empty(t, f.rt.Channels)
}

func TestSendMessage_PublishFailureDoesNotFailRequest(t *testing.T) {
	f := newMsgFixture()
	f.rt.Err = errors.New("realtime down")

	resp, apiErr := f.svc.SendMessage(context.Background(), userID, convID, api.SendMessageRequest{Content: "hi"})

	require.Nil(t, apiErr)
	assert.NotNil(t, resp)
}

func TestListMessages(t *testing.T) {
	t.Run("success applies pagination and cap", func(t *testing.T) {
		f := newMsgFixture()
		var gotLimit, gotOffset int
		var gotBefore *time.Time
		f.msgs.ListFn = func(_ context.Context, _ string, before *time.Time, limit, offset int) ([]db.Message, int, error) {
			gotBefore, gotLimit, gotOffset = before, limit, offset
			return []db.Message{{ID: "m1", ConversationID: convID, SenderID: userID, Content: "hi"}}, 1, nil
		}
		resp, apiErr := f.svc.ListMessages(context.Background(), adminID, convID, api.ListMessagesQuery{Page: 3, PageSize: 500, Before: "2024-01-02T03:04:05Z"})
		require.Nil(t, apiErr)
		assert.Equal(t, 100, resp.PageSize)
		assert.Equal(t, 3, resp.Page)
		assert.Equal(t, 100, gotLimit)
		assert.Equal(t, 200, gotOffset)
		require.NotNil(t, gotBefore)
		assert.Len(t, resp.Items, 1)
		assert.Equal(t, 1, resp.Total)
	})
	t.Run("defaults", func(t *testing.T) {
		f := newMsgFixture()
		f.msgs.ListFn = func(_ context.Context, _ string, _ *time.Time, limit, offset int) ([]db.Message, int, error) {
			assert.Equal(t, 20, limit)
			assert.Equal(t, 0, offset)
			return nil, 0, nil
		}
		resp, apiErr := f.svc.ListMessages(context.Background(), userID, convID, api.ListMessagesQuery{})
		require.Nil(t, apiErr)
		assert.NotNil(t, resp.Items)
	})
	t.Run("non participant gets 404", func(t *testing.T) {
		f := newMsgFixture()
		_, apiErr := f.svc.ListMessages(context.Background(), otherID, convID, api.ListMessagesQuery{})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusNotFound, apiErr.Code)
	})
	t.Run("bad cursor", func(t *testing.T) {
		f := newMsgFixture()
		_, apiErr := f.svc.ListMessages(context.Background(), userID, convID, api.ListMessagesQuery{Before: "yesterday"})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.Code)
	})
	t.Run("repo error is 500", func(t *testing.T) {
		f := newMsgFixture()
		f.msgs.ListFn = func(context.Context, string, *time.Time, int, int) ([]db.Message, int, error) {
			return nil, 0, errors.New("boom")
		}
		_, apiErr := f.svc.ListMessages(context.Background(), userID, convID, api.ListMessagesQuery{})
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.Code)
	})
}
