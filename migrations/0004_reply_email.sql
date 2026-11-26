-- +goose Up
-- Whether the student was already emailed about the team messages they have not read: one email per unread streak,
-- the flag is cleared when they read the conversation. Unread messages from before this change count as told.
ALTER TABLE conversation_participants ADD COLUMN reply_emailed boolean NOT NULL DEFAULT false;
UPDATE conversation_participants SET reply_emailed = true WHERE unread_count > 0;
CREATE INDEX idx_participants_reply_due ON conversation_participants (conversation_id)
  WHERE unread_count > 0 AND NOT reply_emailed;

-- +goose Down
DROP INDEX idx_participants_reply_due;
ALTER TABLE conversation_participants DROP COLUMN reply_emailed;
