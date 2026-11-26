package repositories

import (
	"context"
	"time"
)

// ReplyDue is a student to email about team messages they have not read.
type ReplyDue struct {
	ConversationID string
	UserID         string
	Unread         int
}

// ReplyEmailRepository finds the students the team answered and who have not read it.
type ReplyEmailRepository interface {
	// ClaimDue marks as emailed, and returns, at most limit student seats with unread team messages, not emailed
	// yet in this unread streak, whose conversation has been quiet since quietBefore. Two instances never claim the
	// same seat.
	ClaimDue(ctx context.Context, quietBefore time.Time, limit int) ([]ReplyDue, error)
	// Release gives a claimed seat back, when the event could not be published.
	Release(ctx context.Context, conversationID, userID string) error
}

type replyEmailRepository struct{ store *Store }

func NewReplyEmailRepository(store *Store) ReplyEmailRepository {
	return &replyEmailRepository{store: store}
}

func (r *replyEmailRepository) ClaimDue(ctx context.Context, quietBefore time.Time, limit int) ([]ReplyDue, error) {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	// The student's seat is the participant whose id is the conversation's user_id (the team has its own seat).
	rows, err := r.store.Conn(ctx).Query(ctx, `
UPDATE conversation_participants p SET reply_emailed = true
WHERE (p.conversation_id, p.user_id) IN (
  SELECT q.conversation_id, q.user_id
  FROM conversation_participants q JOIN conversations c ON c.id = q.conversation_id AND c.user_id = q.user_id
  WHERE q.unread_count > 0 AND NOT q.reply_emailed AND c.updated_at < $1
  ORDER BY c.updated_at
  LIMIT $2
  FOR UPDATE OF q SKIP LOCKED
)
RETURNING p.conversation_id::text, p.user_id::text, p.unread_count`, quietBefore, limit)
	if err != nil {
		return nil, MapError(err)
	}
	defer rows.Close()
	var out []ReplyDue
	for rows.Next() {
		var d ReplyDue
		if err := rows.Scan(&d.ConversationID, &d.UserID, &d.Unread); err != nil {
			return nil, MapError(err)
		}
		out = append(out, d)
	}
	return out, MapError(rows.Err())
}

func (r *replyEmailRepository) Release(ctx context.Context, conversationID, userID string) error {
	ctx, cancel := QueryTimeout(ctx)
	defer cancel()
	_, err := r.store.Conn(ctx).Exec(ctx,
		`UPDATE conversation_participants SET reply_emailed = false WHERE conversation_id = $1 AND user_id = $2`, conversationID, userID)
	return MapError(err)
}
