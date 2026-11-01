-- +goose Up
CREATE INDEX card_activity_moved_created_idx
ON card_activity (created_at DESC, id DESC)
WHERE action = 'moved';

-- +goose Down
DROP INDEX IF EXISTS card_activity_moved_created_idx;
