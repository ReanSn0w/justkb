-- +goose Up
ALTER TABLE attachments
    ALTER COLUMN card_id DROP NOT NULL,
    ADD COLUMN epic_id uuid REFERENCES epics(id) ON DELETE CASCADE,
    ADD CONSTRAINT attachments_exactly_one_owner CHECK (
        (card_id IS NOT NULL AND epic_id IS NULL)
        OR (card_id IS NULL AND epic_id IS NOT NULL)
    );

CREATE INDEX attachments_epic_idx ON attachments (epic_id, created_at, id)
WHERE epic_id IS NOT NULL;

-- +goose Down
DELETE FROM attachments WHERE epic_id IS NOT NULL;
DROP INDEX IF EXISTS attachments_epic_idx;
ALTER TABLE attachments DROP CONSTRAINT IF EXISTS attachments_exactly_one_owner;
ALTER TABLE attachments DROP COLUMN IF EXISTS epic_id;
ALTER TABLE attachments ALTER COLUMN card_id SET NOT NULL;
