//go:build integration

package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplyEmailClaimsOncePerUnreadStreak(t *testing.T) {
	store := newTestStore(t)
	convs, msgs, replies := NewConversationRepository(store), NewMessageRepository(store), NewReplyEmailRepository(store)
	ctx := context.Background()
	c, err := convs.GetOrCreate(ctx, itAdmin, itUser)
	require.NoError(t, err)
	later := time.Now().Add(time.Hour)

	due, err := replies.ClaimDue(ctx, later, 10)
	require.NoError(t, err)
	assert.Empty(t, due, "nothing unread, nothing to tell")

	// The team writes twice: the student's seat has 2 unread.
	for range 2 {
		_, err = msgs.Create(ctx, c.ID, itAdmin, "Bonjour")
		require.NoError(t, err)
		require.NoError(t, convs.Touch(ctx, c.ID))
		require.NoError(t, convs.IncrementUnreadForOthers(ctx, c.ID, itAdmin))
	}

	due, err = replies.ClaimDue(ctx, time.Now().Add(-time.Hour), 10)
	require.NoError(t, err)
	assert.Empty(t, due, "a conversation still active waits")

	due, err = replies.ClaimDue(ctx, later, 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	assert.Equal(t, ReplyDue{ConversationID: c.ID, UserID: itUser, Unread: 2}, due[0], "the student's seat only, never the team's")

	due, err = replies.ClaimDue(ctx, later, 10)
	require.NoError(t, err)
	assert.Empty(t, due, "one email per unread streak")

	require.NoError(t, replies.Release(ctx, c.ID, itUser))
	due, err = replies.ClaimDue(ctx, later, 10)
	require.NoError(t, err)
	assert.Len(t, due, 1, "a seat given back is claimed again")

	// Reading the conversation starts a new streak.
	require.NoError(t, convs.MarkRead(ctx, c.ID, itUser))
	_, err = msgs.Create(ctx, c.ID, itAdmin, "Autre chose ?")
	require.NoError(t, err)
	require.NoError(t, convs.IncrementUnreadForOthers(ctx, c.ID, itAdmin))
	due, err = replies.ClaimDue(ctx, later, 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	assert.Equal(t, 1, due[0].Unread)
}
