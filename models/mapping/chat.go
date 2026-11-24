package mapping

import (
	"9itchi-chat-api/models/api"
	"9itchi-chat-api/models/db"
)

func ConversationSummaryToResponse(s db.ConversationSummary) api.ConversationResponse {
	resp := api.ConversationResponse{
		ID:          s.ID,
		AdminID:     s.AdminID,
		UserID:      s.UserID,
		UnreadCount: s.UnreadCount,
		UpdatedAt:   s.UpdatedAt,
		CreatedAt:   s.CreatedAt,
		ResolvedAt:  s.ResolvedAt,
	}
	if s.LastMessageContent != nil && s.LastMessageSenderID != nil && s.LastMessageAt != nil {
		resp.LastMessage = &api.MessagePreview{
			Content:   *s.LastMessageContent,
			SenderID:  *s.LastMessageSenderID,
			CreatedAt: *s.LastMessageAt,
		}
	}
	return resp
}

func MessageToResponse(m db.Message) api.MessageResponse {
	return api.MessageResponse{
		ID:             m.ID,
		ConversationID: m.ConversationID,
		SenderID:       m.SenderID,
		Content:        m.Content,
		CreatedAt:      m.CreatedAt,
	}
}
