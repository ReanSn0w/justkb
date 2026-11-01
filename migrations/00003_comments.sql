-- +goose Up
CREATE TABLE comments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id uuid REFERENCES cards(id) ON DELETE CASCADE,
    epic_id uuid REFERENCES epics(id) ON DELETE CASCADE,
    author_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    author_token_id uuid REFERENCES api_tokens(id) ON DELETE SET NULL,
    body text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT comments_body_not_blank CHECK (btrim(body) <> ''),
    CONSTRAINT comments_exactly_one_owner CHECK (
        (card_id IS NOT NULL AND epic_id IS NULL)
        OR (card_id IS NULL AND epic_id IS NOT NULL)
    )
);

CREATE INDEX comments_card_idx ON comments (card_id, created_at, id)
WHERE card_id IS NOT NULL;
CREATE INDEX comments_epic_idx ON comments (epic_id, created_at, id)
WHERE epic_id IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS comments;
