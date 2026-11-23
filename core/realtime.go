package core

import "context"

// Realtime pushes events to connected clients. channel is the conversation id.
type Realtime interface {
	Publish(ctx context.Context, channel string, payload any) error
}

// NoopRealtime is the default Realtime: it drops every event.
type NoopRealtime struct{}

func (NoopRealtime) Publish(_ context.Context, _ string, _ any) error { return nil }
