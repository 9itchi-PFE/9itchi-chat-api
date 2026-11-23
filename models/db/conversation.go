package db

import "time"

// Conversation maps table conversations.
type Conversation struct {
	ID           string         `db:"id" json:"id"`
	AdminID      string         `db:"admin_id" json:"admin_id"`
	UserID       string         `db:"user_id" json:"user_id"`
	UnreadCounts map[string]int `db:"unread_counts" json:"unread_counts"`
	UpdatedAt    time.Time      `db:"updated_at" json:"updated_at"`
	CreatedAt    time.Time      `db:"created_at" json:"created_at"`
}
