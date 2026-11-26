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
	// Erase removes what this service keeps about a deleted account.
	Erase        EraseService
	Conversation ConversationService
	Message      MessageService
	// ReplyNotifier emails (through notification-api) students the team answered; app.Run starts it.
	ReplyNotifier *ReplyNotifier
	Clients       *Clients
}

// Clients holds one HTTP client per peer service (singletons, 50s timeout).
type Clients struct {
	Account      *account.Client
	Notification *notification.Client
}

func InitServices(config utils.AppConfig, repos *repositories.Repositories) *Services {
	realtime, tokens := core.NewRealtime(config.AblyAPIKey)
	return &Services{
		Erase:         NewEraseService(repos.Erase),
		Conversation:  NewConversationService(repos.Conversation, tokens),
		Message:       NewMessageService(repos.Store, repos.Conversation, repos.Message, realtime),
		ReplyNotifier: NewReplyNotifier(repos.ReplyEmail, core.NewPublisher(config.RabbitMQURL)),
		Clients: &Clients{
			Account:      account.New(config.AccountAPIURL, config.InternalAPIKey),
			Notification: notification.New(config.NotificationAPIURL, config.InternalAPIKey),
		},
	}
}
