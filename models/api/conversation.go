package api

import (
	"time"

	"github.com/ably/ably-go/ably"
)

// StartConversationRequest is the validated body of StartConversation.
type StartConversationRequest struct {
	UserID string `json:"user_id" binding:"required,uuid"`
}

// ListQuery holds pagination query params (page default 1, page_size default 20, max 100).
type ListQuery struct {
	Page     int `form:"page"`
	PageSize int `form:"page_size"`
}

// MessagePreview is the last message of a conversation.
type MessagePreview struct {
	Content   string    `json:"content"`
	SenderID  string    `json:"sender_id"`
	CreatedAt time.Time `json:"created_at"`
}

// ConversationResponse is one conversation as seen by the caller.
type ConversationResponse struct {
	ID          string          `json:"id"`
	AdminID     string          `json:"admin_id"`
	UserID      string          `json:"user_id"`
	UnreadCount int             `json:"unread_count"`
	LastMessage *MessagePreview `json:"last_message"`
	UpdatedAt   time.Time       `json:"updated_at"`
	CreatedAt   time.Time       `json:"created_at"`
}

// ConversationListResponse is a page of conversations.
type ConversationListResponse struct {
	Items    []ConversationResponse `json:"items"`
	Total    int                    `json:"total"`
	Page     int                    `json:"page"`
	PageSize int                    `json:"page_size"`
}

// ConversationReadResponse confirms the unread count reset.
type ConversationReadResponse struct {
	ConversationID string `json:"conversation_id"`
	UnreadCount    int    `json:"unread_count"`
}

// AblyTokenResponse is the signed Ably TokenRequest, returned as-is so the Ably client can use it as authUrl output.
type AblyTokenResponse = ably.TokenRequest

// UnreadCountResponse is the caller's total unread messages.
type UnreadCountResponse struct {
	UnreadCount int `json:"unread_count"`
}
