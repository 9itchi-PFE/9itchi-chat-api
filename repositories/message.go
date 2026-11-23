package repositories

// MessageRepository is the only layer that touches PostgreSQL for message data.
// Use store.Conn(ctx), a context from QueryTimeout, and return MapError(err).
type MessageRepository interface {
}

type messageRepository struct {
	store *Store
}

func NewMessageRepository(store *Store) MessageRepository {
	return &messageRepository{store: store}
}
