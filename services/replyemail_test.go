package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"9itchi-chat-api/core"
	"9itchi-chat-api/repositories"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeReplyRepo struct {
	ClaimDueFn func(ctx context.Context, quietBefore time.Time, limit int) ([]repositories.ReplyDue, error)
	released   []string
}

func (f *fakeReplyRepo) ClaimDue(ctx context.Context, quietBefore time.Time, limit int) ([]repositories.ReplyDue, error) {
	return f.ClaimDueFn(ctx, quietBefore, limit)
}

func (f *fakeReplyRepo) Release(_ context.Context, conversationID, _ string) error {
	f.released = append(f.released, conversationID)
	return nil
}

type fakePublisher struct {
	fail   map[string]bool // by conversation id
	events []map[string]any
}

func (p *fakePublisher) Publish(_ context.Context, event string, payload any) error {
	m := payload.(map[string]any)
	if p.fail[m["conversation_id"].(string)] {
		return errors.New("broker down")
	}
	m["event"] = event
	p.events = append(p.events, m)
	return nil
}

func TestReplyNotifierSweep(t *testing.T) {
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	var asked time.Time
	repo := &fakeReplyRepo{ClaimDueFn: func(_ context.Context, quietBefore time.Time, limit int) ([]repositories.ReplyDue, error) {
		asked = quietBefore
		assert.Equal(t, replyBatch, limit)
		return []repositories.ReplyDue{{ConversationID: "c1", UserID: "u1", Unread: 2}, {ConversationID: "c2", UserID: "u2", Unread: 1}}, nil
	}}
	pub := &fakePublisher{fail: map[string]bool{"c2": true}}
	n := NewReplyNotifier(repo, pub)
	n.now = func() time.Time { return now }

	assert.Equal(t, 1, n.Sweep(context.Background()))
	assert.Equal(t, now.Add(-replyQuiet), asked, "only conversations quiet for a while")
	require.Len(t, pub.events, 1)
	assert.Equal(t, map[string]any{"event": core.EventSupportReplied, "user_id": "u1", "conversation_id": "c1", "unread": 2}, pub.events[0])
	assert.Equal(t, []string{"c2"}, repo.released, "a seat whose event did not leave is tried again later")

	t.Run("a database error sends nothing", func(t *testing.T) {
		repo := &fakeReplyRepo{ClaimDueFn: func(context.Context, time.Time, int) ([]repositories.ReplyDue, error) { return nil, errors.New("down") }}
		pub := &fakePublisher{}
		assert.Zero(t, NewReplyNotifier(repo, pub).Sweep(context.Background()))
		assert.Empty(t, pub.events)
	})
}
