-- +goose Up
-- When the team marked a conversation as resolved. Any new message opens it again (NULL).
ALTER TABLE conversations ADD COLUMN resolved_at timestamptz;

-- +goose Down
ALTER TABLE conversations DROP COLUMN resolved_at;
