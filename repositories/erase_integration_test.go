//go:build integration

package repositories

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Everything of the deleted user goes, in one transaction, and nothing of anyone else.
func TestEraseUser(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	repo := NewEraseRepository(store)
	const gone, kept = "00000000-0000-4000-8000-0000000000e1", "00000000-0000-4000-8000-0000000000e2"
	const count = `SELECT (SELECT count(*) FROM conversations WHERE user_id = $1) + (SELECT count(*) FROM messages WHERE sender_id = $1) + (SELECT count(*) FROM conversation_participants WHERE user_id = $1)`
	left := func(id string) int {
		var n int
		require.NoError(t, store.Conn(ctx).QueryRow(ctx, count, id).Scan(&n))
		return n
	}
	for _, id := range []string{gone, kept} {
		for _, q := range []string{
			`WITH c AS (INSERT INTO conversations (admin_id, user_id) VALUES (gen_random_uuid(), $1) RETURNING id), p AS (INSERT INTO conversation_participants (conversation_id, user_id) SELECT id, $1 FROM c) INSERT INTO messages (conversation_id, sender_id, content) SELECT id, $1, 'bonjour' FROM c`,
		} {
			_, err := store.Conn(ctx).Exec(ctx, q, id)
			require.NoError(t, err, q)
		}
	}
	require.Equal(t, 3, left(gone))

	rows, err := repo.EraseUser(ctx, gone)
	require.NoError(t, err)
	assert.Positive(t, rows)
	assert.Zero(t, left(gone))
	assert.Equal(t, 3, left(kept), "another student keeps everything")

	rows, err = repo.EraseUser(ctx, gone)
	require.NoError(t, err, "erasing again is not an error")
	assert.Zero(t, rows)
}
