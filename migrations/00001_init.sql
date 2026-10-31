-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE user_role AS ENUM ('admin', 'member');

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username text NOT NULL,
    display_name text NOT NULL,
    email text,
    avatar_url text,
    role user_role NOT NULL DEFAULT 'member',
    is_active boolean NOT NULL DEFAULT true,
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT users_username_not_blank CHECK (btrim(username) <> ''),
    CONSTRAINT users_display_name_not_blank CHECK (btrim(display_name) <> '')
);
CREATE UNIQUE INDEX users_username_unique_ci ON users (lower(username));
CREATE UNIQUE INDEX users_email_unique_ci ON users (lower(email)) WHERE email IS NOT NULL;

CREATE TABLE user_groups (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    color text NOT NULL,
    description text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT user_groups_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT user_groups_color_hex CHECK (color ~ '^#[0-9A-Fa-f]{6}$')
);
CREATE UNIQUE INDEX user_groups_name_unique_ci ON user_groups (lower(name));

CREATE TABLE group_members (
    group_id uuid NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, user_id)
);
CREATE INDEX group_members_user_idx ON group_members (user_id, group_id);

CREATE TABLE board_columns (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    position integer NOT NULL,
    wip_limit integer,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT board_columns_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT board_columns_position_positive CHECK (position > 0),
    CONSTRAINT board_columns_wip_limit_positive CHECK (wip_limit IS NULL OR wip_limit > 0),
    UNIQUE (position)
);
CREATE UNIQUE INDEX board_columns_name_unique_ci ON board_columns (lower(name));

CREATE TABLE epics (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    description text,
    color text NOT NULL,
    is_archived boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT epics_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT epics_color_hex CHECK (color ~ '^#[0-9A-Fa-f]{6}$')
);
CREATE UNIQUE INDEX epics_active_name_unique_ci ON epics (lower(name)) WHERE NOT is_archived;
CREATE INDEX epics_archived_name_idx ON epics (is_archived, lower(name));

CREATE TABLE tags (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL,
    color text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tags_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT tags_color_hex CHECK (color ~ '^#[0-9A-Fa-f]{6}$')
);
CREATE UNIQUE INDEX tags_name_unique_ci ON tags (lower(name));

CREATE TABLE cards (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    number bigint GENERATED ALWAYS AS IDENTITY UNIQUE,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    column_id uuid NOT NULL REFERENCES board_columns(id) ON DELETE RESTRICT,
    epic_id uuid REFERENCES epics(id) ON DELETE SET NULL,
    group_id uuid REFERENCES user_groups(id) ON DELETE SET NULL,
    assignee_id uuid REFERENCES users(id) ON DELETE SET NULL,
    position numeric(30, 15) NOT NULL,
    due_at timestamptz,
    version bigint NOT NULL DEFAULT 1,
    is_archived boolean NOT NULL DEFAULT false,
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT cards_title_not_blank CHECK (btrim(title) <> ''),
    CONSTRAINT cards_version_positive CHECK (version > 0)
);
CREATE INDEX cards_board_order_idx ON cards (is_archived, column_id, position, id);
CREATE INDEX cards_epic_idx ON cards (epic_id, is_archived, column_id);
CREATE INDEX cards_group_idx ON cards (group_id, is_archived, column_id);
CREATE INDEX cards_assignee_idx ON cards (assignee_id, is_archived, column_id);
CREATE INDEX cards_search_idx ON cards USING gin (to_tsvector('simple', title || ' ' || description));

CREATE TABLE card_tags (
    card_id uuid NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    tag_id uuid NOT NULL REFERENCES tags(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (card_id, tag_id)
);
CREATE INDEX card_tags_tag_idx ON card_tags (tag_id, card_id);

CREATE TABLE attachments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id uuid NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    original_name text NOT NULL,
    storage_name text NOT NULL UNIQUE,
    media_type text NOT NULL,
    size bigint NOT NULL,
    created_by uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT attachments_original_name_not_blank CHECK (btrim(original_name) <> ''),
    CONSTRAINT attachments_storage_name_not_blank CHECK (btrim(storage_name) <> ''),
    CONSTRAINT attachments_size_nonnegative CHECK (size >= 0)
);
CREATE INDEX attachments_card_idx ON attachments (card_id, created_at, id);

CREATE TABLE web_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_digest bytea NOT NULL UNIQUE,
    csrf_digest bytea NOT NULL,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX web_sessions_user_idx ON web_sessions (user_id, expires_at);
CREATE INDEX web_sessions_expiry_idx ON web_sessions (expires_at);

CREATE TABLE api_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name text NOT NULL,
    token_prefix text NOT NULL,
    token_digest bytea NOT NULL UNIQUE,
    scopes text[] NOT NULL,
    expires_at timestamptz,
    last_used_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT api_tokens_name_not_blank CHECK (btrim(name) <> ''),
    CONSTRAINT api_tokens_scopes_known CHECK (scopes <@ ARRAY['cards:read', 'cards:write']::text[]),
    CONSTRAINT api_tokens_scopes_count CHECK (cardinality(scopes) BETWEEN 1 AND 2)
);
CREATE INDEX api_tokens_user_idx ON api_tokens (user_id, created_at DESC);
CREATE INDEX api_tokens_active_idx ON api_tokens (token_digest) WHERE revoked_at IS NULL;

CREATE TABLE card_activity (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    card_id uuid NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    actor_user_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    actor_token_id uuid REFERENCES api_tokens(id) ON DELETE SET NULL,
    action text NOT NULL,
    changes jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT card_activity_action_not_blank CHECK (btrim(action) <> ''),
    CONSTRAINT card_activity_changes_object CHECK (jsonb_typeof(changes) = 'object')
);
CREATE INDEX card_activity_card_idx ON card_activity (card_id, created_at DESC, id DESC);

INSERT INTO board_columns (name, position)
VALUES ('Backlog', 100), ('In Progress', 200), ('Review', 300), ('Done', 400)
ON CONFLICT DO NOTHING;

-- +goose Down
DROP TABLE IF EXISTS card_activity;
DROP TABLE IF EXISTS api_tokens;
DROP TABLE IF EXISTS web_sessions;
DROP TABLE IF EXISTS attachments;
DROP TABLE IF EXISTS card_tags;
DROP TABLE IF EXISTS cards;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS epics;
DROP TABLE IF EXISTS board_columns;
DROP TABLE IF EXISTS group_members;
DROP TABLE IF EXISTS user_groups;
DROP TABLE IF EXISTS users;
DROP TYPE IF EXISTS user_role;
