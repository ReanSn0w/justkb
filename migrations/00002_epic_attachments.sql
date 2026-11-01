-- +goose Up
CREATE INDEX attachments_epic_idx ON attachments (epic_id, created_at, id)
WHERE epic_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS attachments_epic_idx;
