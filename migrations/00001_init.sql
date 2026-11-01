-- +goose Up
CREATE TABLE users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL,
    display_name TEXT NOT NULL,
    email TEXT,
    avatar_url TEXT,
    role TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member')),
    is_active INTEGER NOT NULL DEFAULT 1 CHECK (is_active IN (0, 1)),
    password_hash TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (trim(username) <> ''),
    CHECK (trim(display_name) <> '')
);
CREATE UNIQUE INDEX users_username_unique_ci ON users (lower(username));
CREATE UNIQUE INDEX users_email_unique_ci ON users (lower(email)) WHERE email IS NOT NULL;

CREATE TABLE user_groups (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    color TEXT NOT NULL,
    description TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (trim(name) <> ''),
    CHECK (color GLOB '#[0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f]')
);
CREATE UNIQUE INDEX user_groups_name_unique_ci ON user_groups (lower(name));

CREATE TABLE group_members (
    group_id TEXT NOT NULL REFERENCES user_groups(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (group_id, user_id)
);
CREATE INDEX group_members_user_idx ON group_members (user_id, group_id);

CREATE TABLE board_columns (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    position INTEGER NOT NULL,
    wip_limit INTEGER,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (trim(name) <> ''),
    CHECK (position > 0),
    CHECK (wip_limit IS NULL OR wip_limit > 0),
    UNIQUE (position)
);
CREATE UNIQUE INDEX board_columns_name_unique_ci ON board_columns (lower(name));

CREATE TABLE epics (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    color TEXT NOT NULL,
    is_archived INTEGER NOT NULL DEFAULT 0 CHECK (is_archived IN (0, 1)),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (trim(name) <> ''),
    CHECK (color GLOB '#[0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f]')
);
CREATE UNIQUE INDEX epics_active_name_unique_ci ON epics (lower(name)) WHERE is_archived = 0;
CREATE INDEX epics_archived_name_idx ON epics (is_archived, lower(name));

CREATE TABLE tags (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    color TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (trim(name) <> ''),
    CHECK (color GLOB '#[0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f][0-9A-Fa-f]')
);
CREATE UNIQUE INDEX tags_name_unique_ci ON tags (lower(name));

CREATE TABLE card_number_sequence (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    next_number INTEGER NOT NULL CHECK (next_number > 0)
);
INSERT INTO card_number_sequence(singleton, next_number) VALUES (1, 1);

CREATE TABLE cards (
    id TEXT PRIMARY KEY,
    number INTEGER NOT NULL UNIQUE,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    column_id TEXT NOT NULL REFERENCES board_columns(id) ON DELETE RESTRICT,
    epic_id TEXT REFERENCES epics(id) ON DELETE SET NULL,
    group_id TEXT REFERENCES user_groups(id) ON DELETE SET NULL,
    assignee_id TEXT REFERENCES users(id) ON DELETE SET NULL,
    position REAL NOT NULL,
    due_at DATETIME,
    version INTEGER NOT NULL DEFAULT 1,
    is_archived INTEGER NOT NULL DEFAULT 0 CHECK (is_archived IN (0, 1)),
    created_by TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (trim(title) <> ''),
    CHECK (version > 0)
);
CREATE INDEX cards_board_order_idx ON cards (is_archived, column_id, position, id);
CREATE INDEX cards_epic_idx ON cards (epic_id, is_archived, column_id);
CREATE INDEX cards_group_idx ON cards (group_id, is_archived, column_id);
CREATE INDEX cards_assignee_idx ON cards (assignee_id, is_archived, column_id);

CREATE TABLE card_tags (
    card_id TEXT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    tag_id TEXT NOT NULL REFERENCES tags(id) ON DELETE RESTRICT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (card_id, tag_id)
);
CREATE INDEX card_tags_tag_idx ON card_tags (tag_id, card_id);

CREATE TABLE attachments (
    id TEXT PRIMARY KEY,
    card_id TEXT REFERENCES cards(id) ON DELETE CASCADE,
    epic_id TEXT REFERENCES epics(id) ON DELETE CASCADE,
    original_name TEXT NOT NULL,
    storage_name TEXT NOT NULL UNIQUE,
    media_type TEXT NOT NULL,
    size INTEGER NOT NULL,
    created_by TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (trim(original_name) <> ''),
    CHECK (trim(storage_name) <> ''),
    CHECK (size >= 0),
    CHECK ((card_id IS NOT NULL) <> (epic_id IS NOT NULL))
);
CREATE INDEX attachments_card_idx ON attachments (card_id, created_at, id) WHERE card_id IS NOT NULL;

CREATE TABLE web_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_digest BLOB NOT NULL UNIQUE,
    csrf_digest BLOB NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX web_sessions_user_idx ON web_sessions (user_id, expires_at);
CREATE INDEX web_sessions_expiry_idx ON web_sessions (expires_at);

CREATE TABLE api_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    token_prefix TEXT NOT NULL,
    token_digest BLOB NOT NULL UNIQUE,
    expires_at DATETIME,
    last_used_at DATETIME,
    revoked_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (trim(name) <> '')
);
CREATE INDEX api_tokens_user_idx ON api_tokens (user_id, created_at DESC);
CREATE INDEX api_tokens_active_idx ON api_tokens (token_digest) WHERE revoked_at IS NULL;

CREATE TABLE api_token_scopes (
    token_id TEXT NOT NULL REFERENCES api_tokens(id) ON DELETE CASCADE,
    scope TEXT NOT NULL CHECK (scope IN ('cards:read', 'cards:write')),
    PRIMARY KEY (token_id, scope)
);

CREATE TABLE card_activity (
    id TEXT PRIMARY KEY,
    card_id TEXT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
    actor_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    actor_token_id TEXT REFERENCES api_tokens(id) ON DELETE SET NULL,
    action TEXT NOT NULL,
    changes TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(changes)),
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CHECK (trim(action) <> '')
);
CREATE INDEX card_activity_card_idx ON card_activity (card_id, created_at DESC, id DESC);

INSERT INTO board_columns (id, name, position) VALUES
    ('00000000-0000-4000-8000-000000000001', 'Backlog', 100),
    ('00000000-0000-4000-8000-000000000002', 'In Progress', 200),
    ('00000000-0000-4000-8000-000000000003', 'Review', 300),
    ('00000000-0000-4000-8000-000000000004', 'Done', 400);

-- +goose Down
DROP TABLE IF EXISTS card_activity;
DROP TABLE IF EXISTS api_token_scopes;
DROP TABLE IF EXISTS api_tokens;
DROP TABLE IF EXISTS web_sessions;
DROP TABLE IF EXISTS attachments;
DROP TABLE IF EXISTS card_tags;
DROP TABLE IF EXISTS cards;
DROP TABLE IF EXISTS card_number_sequence;
DROP TABLE IF EXISTS tags;
DROP TABLE IF EXISTS epics;
DROP TABLE IF EXISTS board_columns;
DROP TABLE IF EXISTS group_members;
DROP TABLE IF EXISTS user_groups;
DROP TABLE IF EXISTS users;
