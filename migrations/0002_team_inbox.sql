-- +goose Up
-- Support becomes a team inbox: one conversation per student, held by the team as a whole instead of one admin.
-- The team has one seat (a fixed id) with one unread count; the admin who writes stays the sender of the message.

-- Per student, the conversation updated last is kept; the messages of the others move into it.
WITH keep AS (
  SELECT DISTINCT ON (user_id) id, user_id FROM conversations ORDER BY user_id, updated_at DESC
)
UPDATE messages m SET conversation_id = k.id
FROM conversations c JOIN keep k ON k.user_id = c.user_id
WHERE m.conversation_id = c.id AND c.id <> k.id;

WITH keep AS (
  SELECT DISTINCT ON (user_id) id, user_id FROM conversations ORDER BY user_id, updated_at DESC
)
DELETE FROM conversations c USING keep k WHERE c.user_id = k.user_id AND c.id <> k.id;

-- The seat of each admin becomes the seat of the team, with what the admin had not read yet.
INSERT INTO conversation_participants (conversation_id, user_id, unread_count)
SELECT c.id, '00000000-0000-4000-8000-000000000000', COALESCE(p.unread_count, 0)
FROM conversations c LEFT JOIN conversation_participants p ON p.conversation_id = c.id AND p.user_id = c.admin_id
ON CONFLICT (conversation_id, user_id) DO NOTHING;
DELETE FROM conversation_participants p USING conversations c
WHERE p.conversation_id = c.id AND p.user_id = c.admin_id AND c.admin_id <> '00000000-0000-4000-8000-000000000000';
UPDATE conversations SET admin_id = '00000000-0000-4000-8000-000000000000';

-- +goose Down
-- Which admin held which conversation is not kept: nothing to put back.
SELECT 1;
