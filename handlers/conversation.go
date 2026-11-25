package handlers

import (
	"net/http"

	"9itchi-chat-api/models"
	"9itchi-chat-api/models/api"
	"9itchi-chat-api/services"

	"github.com/gin-gonic/gin"
)

// ListConversations: GET /conversations (user) conversations the caller takes part in.
func ListConversations(svc services.ConversationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var query api.ListQuery
		if err := c.ShouldBindQuery(&query); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.ListConversations(c.Request.Context(), c.GetString("seat_id"), query)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// StartConversation: POST /admin/conversations (admin) admin starts a chat with any user (idempotent, returns the existing one).
func StartConversation(svc services.ConversationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.StartConversationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.StartConversation(c.Request.Context(), c.GetString("user_id"), c.GetString("role"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// MarkConversationRead: PUT /conversations/:id/read (user) reset unread count.
func MarkConversationRead(svc services.ConversationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.MarkConversationRead(c.Request.Context(), c.GetString("seat_id"), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// GlobalUnreadCount: GET /user/global-unread-count (user) total unread across conversations.
func GlobalUnreadCount(svc services.ConversationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.GlobalUnreadCount(c.Request.Context(), c.GetString("seat_id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// AblyToken: GET /chat/ably-token (user) scoped Ably token request.
func AblyToken(svc services.ConversationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.AblyToken(c.Request.Context(), c.GetString("seat_id"), c.GetString("user_id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// OpenSupport: POST /conversations (user) the student's conversation with the team, created on first use.
func OpenSupport(svc services.ConversationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.OpenSupport(c.Request.Context(), c.GetString("user_id"), c.GetString("role"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// SetResolved: PUT /admin/conversations/:id/resolved (admin) mark a conversation as resolved, or open again.
func SetResolved(svc services.ConversationService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.ResolveRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.SetResolved(c.Request.Context(), c.Param("id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}
