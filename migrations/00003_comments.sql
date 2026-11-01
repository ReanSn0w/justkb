-- +goose Up
CREATE TABLE comments (
    id TEXT PRIMARY KEY,
    card_id TEXT REFERENCES cards(id) ON DELETE CASCADE,
    epic_id TEXT REFERENCES epics(id) ON DELETE CASCADE,
    author_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    author_token_id TEXT REFERENCES api_tokens(id) ON DELETE SET NULL,
    body TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (trim(body) <> ''),
    CHECK ((card_id IS NOT NULL) <> (epic_id IS NOT NULL))
);

CREATE INDEX comments_card_idx ON comments (card_id, created_at, id)
WHERE card_id IS NOT NULL;
CREATE INDEX comments_epic_idx ON comments (epic_id, created_at, id)
WHERE epic_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS comments;
