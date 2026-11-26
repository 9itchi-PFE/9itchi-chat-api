package services

import (
	"context"
	"time"

	"9itchi-chat-api/core"
	"9itchi-chat-api/repositories"
	"9itchi-chat-api/utils"

	"go.uber.org/zap"
)

const (
	// replyQuiet is how long a conversation stays quiet before the student is emailed: a student reading the chat
	// at that moment has read it by then, and a team writing several messages sends one email.
	replyQuiet = 2 * time.Minute
	// replySweepEvery is how often the seats to email are looked for; replyBatch bounds one sweep.
	replySweepEvery = time.Minute
	replyBatch      = 100
)

// ReplyNotifier tells notification-api (event support.replied) about students the team answered and who have not
// read it, once per unread streak. It works from the database, so a restart loses nothing.
type ReplyNotifier struct {
	repo   repositories.ReplyEmailRepository
	events core.Publisher
	now    func() time.Time
}

func NewReplyNotifier(repo repositories.ReplyEmailRepository, events core.Publisher) *ReplyNotifier {
	return &ReplyNotifier{repo: repo, events: events, now: time.Now}
}

// Run sweeps every replySweepEvery until ctx ends.
func (n *ReplyNotifier) Run(ctx context.Context) {
	t := time.NewTicker(replySweepEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			n.Sweep(ctx)
		}
	}
}

// Sweep publishes one event per seat due and returns how many left. A seat whose event could not be published is
// given back, to be tried at the next sweep.
func (n *ReplyNotifier) Sweep(ctx context.Context) int {
	due, err := n.repo.ClaimDue(ctx, n.now().Add(-replyQuiet), replyBatch)
	if err != nil {
		utils.Logger.Warn("support replies not read", zap.Error(err))
		return 0
	}
	sent := 0
	for _, d := range due {
		payload := map[string]any{"user_id": d.UserID, "conversation_id": d.ConversationID, "unread": d.Unread}
		if err := n.events.Publish(ctx, core.EventSupportReplied, payload); err != nil {
			utils.Logger.Warn("support reply event not published", zap.String("conversation_id", d.ConversationID))
			if err := n.repo.Release(context.WithoutCancel(ctx), d.ConversationID, d.UserID); err != nil {
				utils.Logger.Error("support reply seat not given back", zap.String("conversation_id", d.ConversationID), zap.Error(err))
			}
			continue
		}
		sent++
	}
	return sent
}
