package repositories

import (
	"context"

	"9itchi-chat-api/models/db"

	"github.com/jackc/pgx/v5"
)

// ConversationRepository is the only layer that touches PostgreSQL for conversation data.
// Use store.Conn(ctx), a context from QueryTimeout, and return MapError(err).
type ConversationRepository interface {
	// GetOrCreate returns the conversation of the (admin, user) pair, creating it and both participant rows if missing.
	GetOrCreate(ctx context.Context, adminID, userID string) (*db.Conversation, error)
	// GetForParticipant returns the conversation as seen by userID; ErrNotFound if userID is not a participant.
	GetForParticipant(ctx context.Context, id, userID string) (*db.ConversationSummary, error)
	// ListForUser returns the caller's conversations (updated_at desc) and the total count.
	ListForUser(ctx context.Context, userID string, limit, offset int) ([]db.ConversationSummary, int, error)
	// MarkRead resets the caller's unread count; ErrNotFound if userID is not a participant.
	MarkRead(ctx context.Context, id, userID string) error
	// GlobalUnreadCount sums the caller's unread counts in one aggregate query.
	GlobalUnreadCount(ctx context.Context, userID string) (int, error)
	// Touch bumps conversations.updated_at when a message is written; a resolved conversation is open again.
	Touch(ctx context.Context, id string) error
	// SetResolved marks a conversation as resolved, or open again; ErrNotFound if it does not exist.
	SetResolved(ctx context.Context, id string, resolved bool) error
	// IncrementUnreadForOthers atomically adds 1 to every participant except senderID.
	IncrementUnreadForOthers(ctx context.Context, id, senderID string) error
}

type conversationRepository struct {
	store *Store
}

func NewConversationRepository(store *Store) ConversationRepository {
	return &conversationRepository{store: store}
}

const summarySelect = `
SELECT c.id, c.admin_id, c.user_id, c.updated_at, c.created_at, c.resolved_at, p.unread_count,
       lm.content AS last_message_content, lm.sender_id AS last_message_sender_id, lm.created_at AS last_message_at
FROM conversation_participants p
JOIN conversations c ON c.id = p.conversation_id
LEFT JOIN LATERAL (
  SELECT left(content, 200) AS content, sender_id, created_at
  FROM messages WHERE conversation_id = c.id
  ORDER BY created_at DESC, id DESC LIMIT 1
) lm ON true
WHERE p.user_id = $1`

func (r *conversationRepository) GetOrCreate(ctx context.Context, adminID, userID string) (*db.Conversation, error) {
	var conv *db.Conversation
	err := r.store.WithTx(ctx, func(ctx context.Context) error {
		ctx, cancel := QueryTimeout(ctx)
		defer cancel()
		conn := r.store.Conn(ctx)
		// DO UPDATE no-op so RETURNING yields the row for both new and existing pairs.
		rows, err := conn.Query(ctx, `
INSERT INTO conversations (admin_id, user_id) VALUES ($1, $2)
ON CONFLICT (admin_id, user_id) DO UPDATE SET admin_id = EXCLUDED.admin_id
RETURNING id, admin_id, user_id, updated_at, created_at`, adminID, userID)
		if err != nil {
			return MapError(err)
		}
		conv, err = pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.Conversation])
		if err != nil {
			return MapError(err)
		}
		_, err = conn.Exec(ctx, `
INSERT INTO conversation_participants (conversation_id, user_id) VALUES ($1, $2), ($1, $3)
ON CONFLICT DO NOTHING`, conv.ID, adminID, userID)
		return MapError(err)
	})
	if err != nil {
		return nil, err
	}
	return conv, nil
}

func (r *conversationRepository) GetForParticipant(ctx context.Context, id, userID string) (*db.ConversationSummary, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	rows, err := r.store.Conn(ctx).Query(ctx, summarySelect+` AND c.id = $2`, userID, id)
	if err != nil {
		return nil, MapError(err)
	}
	s, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[db.ConversationSummary])
	return s, MapError(err)
}

func (r *conversationRepository) ListForUser(ctx context.Context, userID string, limit, offset int) ([]db.ConversationSummary, int, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	conn := r.store.Conn(ctx)
	var total int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM conversation_participants WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, MapError(err)
	}
	rows, err := conn.Query(ctx, summarySelect+` ORDER BY c.updated_at DESC, c.id DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, MapError(err)
	}
	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[db.ConversationSummary])
	if err != nil {
		return nil, 0, MapError(err)
	}
	return items, total, nil
}

func (r *conversationRepository) MarkRead(ctx context.Context, id, userID string) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	tag, err := r.store.Conn(ctx).Exec(ctx,
		`UPDATE conversation_participants SET unread_count = 0 WHERE conversation_id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *conversationRepository) GlobalUnreadCount(ctx context.Context, userID string) (int, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	var n int
	err := r.store.Conn(ctx).QueryRow(ctx,
		`SELECT COALESCE(SUM(unread_count), 0)::int FROM conversation_participants WHERE user_id = $1`, userID).Scan(&n)
	return n, MapError(err)
}

func (r *conversationRepository) Touch(ctx context.Context, id string) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	_, err := r.store.Conn(ctx).Exec(ctx, `UPDATE conversations SET updated_at = now(), resolved_at = NULL WHERE id = $1`, id)
	return MapError(err)
}

func (r *conversationRepository) SetResolved(ctx context.Context, id string, resolved bool) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	tag, err := r.store.Conn(ctx).Exec(ctx, `UPDATE conversations SET resolved_at = CASE WHEN $2 THEN now() END WHERE id = $1`, id, resolved)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *conversationRepository) IncrementUnreadForOthers(ctx context.Context, id, senderID string) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	_, err := r.store.Conn(ctx).Exec(ctx,
		`UPDATE conversation_participants SET unread_count = unread_count + 1 WHERE conversation_id = $1 AND user_id <> $2`, id, senderID)
	return MapError(err)
}
