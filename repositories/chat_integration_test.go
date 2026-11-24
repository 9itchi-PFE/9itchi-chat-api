//go:build integration

package repositories

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	itAdmin = "22222222-2222-2222-2222-222222222222"
	itUser  = "33333333-3333-3333-3333-333333333333"
	itOther = "44444444-4444-4444-4444-444444444444"
)

func TestGetOrCreateIsIdempotent(t *testing.T) {
	store := newTestStore(t)
	repo := NewConversationRepository(store)
	ctx := context.Background()

	a, err := repo.GetOrCreate(ctx, itAdmin, itUser)
	require.NoError(t, err)
	b, err := repo.GetOrCreate(ctx, itAdmin, itUser)
	require.NoError(t, err)
	assert.Equal(t, a.ID, b.ID)

	var convs, parts int
	require.NoError(t, store.Conn(ctx).QueryRow(ctx, `SELECT count(*) FROM conversations`).Scan(&convs))
	require.NoError(t, store.Conn(ctx).QueryRow(ctx, `SELECT count(*) FROM conversation_participants`).Scan(&parts))
	assert.Equal(t, 1, convs)
	assert.Equal(t, 2, parts)

	_, err = repo.GetOrCreate(ctx, itAdmin, itAdmin)
	assert.Error(t, err, "admin_id <> user_id check")
}

func TestParticipantIsolationAndRead(t *testing.T) {
	store := newTestStore(t)
	convs, msgs := NewConversationRepository(store), NewMessageRepository(store)
	ctx := context.Background()
	c, err := convs.GetOrCreate(ctx, itAdmin, itUser)
	require.NoError(t, err)

	_, err = convs.GetForParticipant(ctx, c.ID, itOther)
	assert.ErrorIs(t, err, ErrNotFound)
	assert.ErrorIs(t, convs.MarkRead(ctx, c.ID, itOther), ErrNotFound)

	_, err = msgs.Create(ctx, c.ID, itAdmin, "hello")
	require.NoError(t, err)
	require.NoError(t, convs.IncrementUnreadForOthers(ctx, c.ID, itAdmin))

	s, err := convs.GetForParticipant(ctx, c.ID, itUser)
	require.NoError(t, err)
	assert.Equal(t, 1, s.UnreadCount)
	require.NotNil(t, s.LastMessageContent)
	assert.Equal(t, "hello", *s.LastMessageContent)
	sa, err := convs.GetForParticipant(ctx, c.ID, itAdmin)
	require.NoError(t, err)
	assert.Equal(t, 0, sa.UnreadCount, "sender's own count untouched")

	n, err := convs.GlobalUnreadCount(ctx, itUser)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	require.NoError(t, convs.MarkRead(ctx, c.ID, itUser))
	n, err = convs.GlobalUnreadCount(ctx, itUser)
	require.NoError(t, err)
	assert.Equal(t, 0, n)
}

func TestListForUserOrderAndMessagePagination(t *testing.T) {
	store := newTestStore(t)
	convs, msgs := NewConversationRepository(store), NewMessageRepository(store)
	ctx := context.Background()
	c1, err := convs.GetOrCreate(ctx, itAdmin, itUser)
	require.NoError(t, err)
	c2, err := convs.GetOrCreate(ctx, itAdmin, itOther)
	require.NoError(t, err)
	require.NoError(t, convs.Touch(ctx, c1.ID)) // c1 now most recent

	items, total, err := convs.ListForUser(ctx, itAdmin, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	require.Len(t, items, 2)
	assert.Equal(t, c1.ID, items[0].ID)
	assert.Equal(t, c2.ID, items[1].ID)

	items, total, err = convs.ListForUser(ctx, itOther, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	assert.Equal(t, c2.ID, items[0].ID)

	for _, txt := range []string{"a", "b", "c"} {
		_, err = msgs.Create(ctx, c1.ID, itUser, txt)
		require.NoError(t, err)
	}
	list, total, err := msgs.List(ctx, c1.ID, nil, 2, 0)
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	require.Len(t, list, 2)
	newest := list[0].CreatedAt
	assert.Equal(t, "c", list[0].Content, "newest first")
	list, _, err = msgs.List(ctx, c1.ID, nil, 2, 2)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "a", list[0].Content)
	list, total, err = msgs.List(ctx, c1.ID, &newest, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, total, "cursor excludes created_at >= before (newest message)")
	assert.Len(t, list, 2)
}

func TestMessageContentConstraints(t *testing.T) {
	store := newTestStore(t)
	convs, msgs := NewConversationRepository(store), NewMessageRepository(store)
	ctx := context.Background()
	c, err := convs.GetOrCreate(ctx, itAdmin, itUser)
	require.NoError(t, err)

	_, err = msgs.Create(ctx, c.ID, itUser, "")
	assert.Error(t, err)
	_, err = msgs.Create(ctx, c.ID, itUser, string(make([]rune, 4001)))
	assert.Error(t, err)
	_, err = msgs.Create(ctx, "55555555-5555-5555-5555-555555555555", itUser, "x")
	assert.Error(t, err, "FK to conversations")
}

func TestConcurrentSendsKeepUnreadCountExact(t *testing.T) {
	store := newTestStore(t)
	convs, msgs := NewConversationRepository(store), NewMessageRepository(store)
	ctx := context.Background()
	c, err := convs.GetOrCreate(ctx, itAdmin, itUser)
	require.NoError(t, err)

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- store.WithTx(ctx, func(ctx context.Context) error {
				if _, err := msgs.Create(ctx, c.ID, itAdmin, "x"); err != nil {
					return err
				}
				if err := convs.Touch(ctx, c.ID); err != nil {
					return err
				}
				return convs.IncrementUnreadForOthers(ctx, c.ID, itAdmin)
			})
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	got, err := convs.GlobalUnreadCount(ctx, itUser)
	require.NoError(t, err)
	assert.Equal(t, n, got)
	_, total, err := msgs.List(ctx, c.ID, nil, 1, 0)
	require.NoError(t, err)
	assert.Equal(t, n, total)
}

func TestCascadeDeleteRemovesChildren(t *testing.T) {
	store := newTestStore(t)
	convs, msgs := NewConversationRepository(store), NewMessageRepository(store)
	ctx := context.Background()
	c, err := convs.GetOrCreate(ctx, itAdmin, itUser)
	require.NoError(t, err)
	_, err = msgs.Create(ctx, c.ID, itUser, "x")
	require.NoError(t, err)
	_, err = store.Conn(ctx).Exec(ctx, `DELETE FROM conversations WHERE id = $1`, c.ID)
	require.NoError(t, err)
	var p, m int
	require.NoError(t, store.Conn(ctx).QueryRow(ctx, `SELECT count(*) FROM conversation_participants`).Scan(&p))
	require.NoError(t, store.Conn(ctx).QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&m))
	assert.Zero(t, p+m)
}

// The team has one seat in every conversation: any admin reads and answers from it, and a student has one
// conversation with the team whoever opened it.
func TestTeamSeat(t *testing.T) {
	store := newTestStore(t)
	convs, msgs := NewConversationRepository(store), NewMessageRepository(store)
	ctx := context.Background()
	const team = "00000000-0000-4000-8000-000000000000"
	const student, other, anAdmin = "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", "33333333-3333-4333-8333-333333333333"

	first, err := convs.GetOrCreate(ctx, team, student)
	require.NoError(t, err)
	again, err := convs.GetOrCreate(ctx, team, student)
	require.NoError(t, err)
	assert.Equal(t, first.ID, again.ID, "one conversation per student")
	second, err := convs.GetOrCreate(ctx, team, other)
	require.NoError(t, err)

	// The student writes: the team has one unread, the student none.
	_, err = msgs.Create(ctx, first.ID, student, "Bonjour")
	require.NoError(t, err)
	require.NoError(t, convs.IncrementUnreadForOthers(ctx, first.ID, student))
	n, err := convs.GlobalUnreadCount(ctx, team)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	n, err = convs.GlobalUnreadCount(ctx, student)
	require.NoError(t, err)
	assert.Zero(t, n)

	// An admin answers from the seat of the team: the message keeps the admin as its sender, the student gets one unread.
	m, err := msgs.Create(ctx, first.ID, anAdmin, "On regarde")
	require.NoError(t, err)
	assert.Equal(t, anAdmin, m.SenderID)
	require.NoError(t, convs.IncrementUnreadForOthers(ctx, first.ID, team))
	n, err = convs.GlobalUnreadCount(ctx, student)
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	n, err = convs.GlobalUnreadCount(ctx, team)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "the team's own answer does not count as unread for the team")

	// The inbox of the team holds every student; an admin's own id holds nothing.
	list, total, err := convs.ListForUser(ctx, team, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.ElementsMatch(t, []string{first.ID, second.ID}, []string{list[0].ID, list[1].ID})
	_, total, err = convs.ListForUser(ctx, anAdmin, 10, 0)
	require.NoError(t, err)
	assert.Zero(t, total)
	_, err = convs.GetForParticipant(ctx, first.ID, anAdmin)
	assert.ErrorIs(t, err, ErrNotFound)
	_, err = convs.GetForParticipant(ctx, first.ID, other)
	assert.ErrorIs(t, err, ErrNotFound, "another student does not see it")
}

func TestResolvedUntilTheNextMessage(t *testing.T) {
	store := newTestStore(t)
	convs := NewConversationRepository(store)
	ctx := context.Background()
	const team, student = "00000000-0000-4000-8000-000000000000", "11111111-1111-4111-8111-111111111111"
	c, err := convs.GetOrCreate(ctx, team, student)
	require.NoError(t, err)

	require.NoError(t, convs.SetResolved(ctx, c.ID, true))
	s, err := convs.GetForParticipant(ctx, c.ID, team)
	require.NoError(t, err)
	require.NotNil(t, s.ResolvedAt)
	list, _, err := convs.ListForUser(ctx, student, 10, 0)
	require.NoError(t, err)
	require.NotNil(t, list[0].ResolvedAt, "the student's view says so too")

	// A new message (Touch) opens it again.
	require.NoError(t, convs.Touch(ctx, c.ID))
	s, err = convs.GetForParticipant(ctx, c.ID, team)
	require.NoError(t, err)
	assert.Nil(t, s.ResolvedAt)

	require.NoError(t, convs.SetResolved(ctx, c.ID, true))
	require.NoError(t, convs.SetResolved(ctx, c.ID, false))
	s, err = convs.GetForParticipant(ctx, c.ID, team)
	require.NoError(t, err)
	assert.Nil(t, s.ResolvedAt)

	assert.ErrorIs(t, convs.SetResolved(ctx, "99999999-9999-4999-8999-999999999999", true), ErrNotFound)
}
