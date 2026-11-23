package handlers

import (
	"net/http"

	"9itchi-chat-api/models"
	"9itchi-chat-api/models/api"
	"9itchi-chat-api/services"

	"github.com/gin-gonic/gin"
)

// ListMessages: GET /conversations/:id/messages (user) message history.
func ListMessages(svc services.MessageService) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, apiErr := svc.ListMessages(c.Request.Context(), c.GetString("user_id"), c.Param("id"))
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusOK, result)
	}
}

// SendMessage: POST /conversations/:id/messages (user) store then publish on Ably channel (conversationId).
func SendMessage(svc services.MessageService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req api.SendMessageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			models.FormatError(c, &models.ApiError{Code: http.StatusBadRequest, Error: err})
			return
		}
		result, apiErr := svc.SendMessage(c.Request.Context(), c.GetString("user_id"), c.Param("id"), req)
		if apiErr != nil {
			models.FormatError(c, apiErr)
			return
		}
		c.JSON(http.StatusCreated, result)
	}
}
