package repositories

import (
	"context"
	"time"

	"9itchi-chat-api/models/db"

	"github.com/jackc/pgx/v5"
)

// MessageRepository is the only layer that touches PostgreSQL for message data.
// Use store.Conn(ctx), a context from QueryTimeout, and return MapError(err).
type MessageRepository interface {
	Create(ctx context.Context, conversationID, senderID, content string) (*db.Message, error)
	// List returns messages newest first. before (optional) keeps only created_at < before; total honours it.
	List(ctx context.Context, conversationID string, before *time.Time, limit, offset int) ([]db.Message, int, error)
}

type messageRepository struct {
	store *Store
}

func NewMessageRepository(store *Store) MessageRepository {
	return &messageRepository{store: store}
}

func (r *messageRepository) Create(ctx context.Context, conversationID, senderID, content string) (*db.Message, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx, `
INSERT INTO messages (conversation_id, sender_id, content) VALUES ($1, $2, $3)
RETURNING id, conversation_id, sender_id, content, created_at, updated_at`, conversationID, senderID, content)
	if err != nil {
		return nil, MapError(err)
	}
	m, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Message])
	return m, MapError(err)
}

func (r *messageRepository) List(ctx context.Context, conversationID string, before *time.Time, limit, offset int) ([]db.Message, int, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	conn := r.store.Conn(ctx)
	var total int
	if err := conn.QueryRow(ctx,
		`SELECT count(*) FROM messages WHERE conversation_id = $1 AND ($2::timestamptz IS NULL OR created_at < $2)`,
		conversationID, before).Scan(&total); err != nil {
		return nil, 0, MapError(err)
	}
	rows, err := conn.Query(ctx, `
SELECT id, conversation_id, sender_id, content, created_at, updated_at
FROM messages WHERE conversation_id = $1 AND ($2::timestamptz IS NULL OR created_at < $2)
ORDER BY created_at DESC, id DESC LIMIT $3 OFFSET $4`, conversationID, before, limit, offset)
	if err != nil {
		return nil, 0, MapError(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.Message])
	if err != nil {
		return nil, 0, MapError(err)
	}
	return items, total, nil
}
