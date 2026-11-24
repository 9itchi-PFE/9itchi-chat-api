package services

import (
	"9itchi-chat-api/core"
	"9itchi-chat-api/pkg/account"
	"9itchi-chat-api/pkg/notification"
	"9itchi-chat-api/repositories"
	"9itchi-chat-api/utils"
)

// Services holds service implementations and the peer clients they call.
type Services struct {
	Conversation ConversationService
	Message      MessageService
	Clients      *Clients
}

// Clients holds one HTTP client per peer service (singletons, 50s timeout).
type Clients struct {
	Account      *account.Client
	Notification *notification.Client
}

func InitServices(config utils.AppConfig, repos *repositories.Repositories) *Services {
	realtime, tokens := core.NewRealtime(config.AblyAPIKey)
	return &Services{
		Conversation: NewConversationService(repos.Conversation, tokens),
		Message:      NewMessageService(repos.Store, repos.Conversation, repos.Message, realtime),
		Clients: &Clients{
			Account:      account.New(config.AccountAPIURL, config.InternalAPIKey),
			Notification: notification.New(config.NotificationAPIURL, config.InternalAPIKey),
		},
	}
}
