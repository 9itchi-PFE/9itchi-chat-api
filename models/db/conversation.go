package db

import "time"

// Conversation maps table conversations.
type Conversation struct {
	ID        string    `db:"id" json:"id"`
	AdminID   string    `db:"admin_id" json:"admin_id"`
	UserID    string    `db:"user_id" json:"user_id"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// ConversationParticipant maps table conversation_participants.
type ConversationParticipant struct {
	ConversationID string `db:"conversation_id" json:"conversation_id"`
	UserID         string `db:"user_id" json:"user_id"`
	UnreadCount    int    `db:"unread_count" json:"unread_count"`
}

// ConversationSummary is a conversation row joined with the caller's unread count and last message.
type ConversationSummary struct {
	ID                  string     `db:"id"`
	AdminID             string     `db:"admin_id"`
	UserID              string     `db:"user_id"`
	UpdatedAt           time.Time  `db:"updated_at"`
	CreatedAt           time.Time  `db:"created_at"`
	ResolvedAt          *time.Time `db:"resolved_at"`
	UnreadCount         int        `db:"unread_count"`
	LastMessageContent  *string    `db:"last_message_content"`
	LastMessageSenderID *string    `db:"last_message_sender_id"`
	LastMessageAt       *time.Time `db:"last_message_at"`
}
