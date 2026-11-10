-- +goose Up
CREATE TABLE card_additional_assignees (
    card_id TEXT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    position INTEGER NOT NULL CHECK (position > 0),
    PRIMARY KEY (card_id, user_id),
    UNIQUE (card_id, position)
);

CREATE INDEX card_additional_assignees_user_idx ON card_additional_assignees (user_id, card_id);

-- +goose Down
-- Refuse to discard assignments that the older schema cannot represent.
CREATE TEMP TABLE additional_assignees_rollback_guard (
    remaining INTEGER NOT NULL CHECK (remaining = 0)
);
INSERT INTO additional_assignees_rollback_guard SELECT count(*) FROM card_additional_assignees;
DROP TABLE additional_assignees_rollback_guard;
DROP TABLE card_additional_assignees;
