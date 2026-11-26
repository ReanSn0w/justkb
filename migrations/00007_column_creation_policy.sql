-- +goose Up
ALTER TABLE board_columns ADD COLUMN allow_card_creation INTEGER NOT NULL DEFAULT 0 CHECK (allow_card_creation IN (0,1));
UPDATE board_columns SET allow_card_creation=1;

-- +goose Down
ALTER TABLE board_columns DROP COLUMN allow_card_creation;
