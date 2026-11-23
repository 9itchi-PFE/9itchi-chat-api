package db

import "time"

// Message maps table messages.
type Message struct {
	ID             string    `db:"id" json:"id"`
	ConversationID string    `db:"conversation_id" json:"conversation_id"`
	SenderID       string    `db:"sender_id" json:"sender_id"`
	Content        string    `db:"content" json:"content"`
	CreatedAt      time.Time `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time `db:"updated_at" json:"updated_at"`
}
