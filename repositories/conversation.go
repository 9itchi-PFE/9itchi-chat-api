package repositories

// ConversationRepository is the only layer that touches PostgreSQL for conversation data.
// Use store.Conn(ctx), a context from QueryTimeout, and return MapError(err).
type ConversationRepository interface {
}

type conversationRepository struct {
	store *Store
}

func NewConversationRepository(store *Store) ConversationRepository {
	return &conversationRepository{store: store}
}
