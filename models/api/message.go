package api

import "time"

// SendMessageRequest is the validated body of SendMessage (content is trimmed, 1..4000 chars, checked in the service).
type SendMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

// ListMessagesQuery holds pagination query params. Before is an optional RFC3339 cursor on created_at.
type ListMessagesQuery struct {
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
	Before   string `form:"before"`
}

// MessageResponse is one chat message.
type MessageResponse struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	SenderID       string    `json:"sender_id"`
	Content        string    `json:"content"`
	CreatedAt      time.Time `json:"created_at"`
}

// MessageListResponse is a page of messages, newest first.
type MessageListResponse struct {
	Items    []MessageResponse `json:"items"`
	Total    int               `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}
