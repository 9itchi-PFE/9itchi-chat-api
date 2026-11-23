package router

import (
	"9itchi-chat-api/handlers"
	"9itchi-chat-api/services"

	"github.com/gin-gonic/gin"
)

func InitializeRouter(services *services.Services) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	initializeMiddlewares(r)
	initializeRoutes(r, services)

	return r
}

func initializeRoutes(r *gin.Engine, services *services.Services) {
	// Health endpoints
	r.GET("/", handlers.GetHealth)
	r.GET("/health", handlers.GetHealth)

	// user endpoints
	userGroup := r.Group("/")
	userGroup.Use(AuthMiddleware())
	{
		userGroup.GET("/conversations", handlers.ListConversations(services.Conversation))
		userGroup.PUT("/conversations/:id/read", handlers.MarkConversationRead(services.Conversation))
		userGroup.GET("/user/global-unread-count", handlers.GlobalUnreadCount(services.Conversation))
		userGroup.GET("/chat/ably-token", handlers.AblyToken(services.Conversation))
		userGroup.GET("/conversations/:id/messages", handlers.ListMessages(services.Message))
		userGroup.POST("/conversations/:id/messages", handlers.SendMessage(services.Message))
	}

	// admin endpoints
	adminGroup := r.Group("/")
	adminGroup.Use(AuthMiddleware(), RequireRole("admin"))
	{
		adminGroup.POST("/admin/conversations", handlers.StartConversation(services.Conversation))
	}

	// Fallback
	r.NoRoute(handlers.NoRoute)
}
