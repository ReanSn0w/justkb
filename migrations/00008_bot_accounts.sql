-- +goose Up
ALTER TABLE users ADD COLUMN account_type TEXT NOT NULL DEFAULT 'human' CHECK (account_type IN ('human','bot'));

-- Empty hash explicitly represents absence of a bot password. Keep the users
-- table and all existing foreign keys intact rather than rebuilding history.
-- +goose StatementBegin
CREATE TRIGGER users_account_insert BEFORE INSERT ON users BEGIN
 SELECT CASE WHEN (NEW.account_type='bot' AND (NEW.role<>'member' OR NEW.password_hash<>''))
 OR (NEW.account_type='human' AND NEW.password_hash='') THEN RAISE(ABORT,'invalid account credentials or role') END;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER users_account_update BEFORE UPDATE ON users BEGIN
 SELECT CASE WHEN NEW.account_type<>OLD.account_type THEN RAISE(ABORT,'account type is immutable') END;
 SELECT CASE WHEN (NEW.account_type='bot' AND (NEW.role<>'member' OR NEW.password_hash<>''))
 OR (NEW.account_type='human' AND NEW.password_hash='') THEN RAISE(ABORT,'invalid account credentials or role') END;
END;
-- +goose StatementEnd
-- A trigger can consult the owner table; a SQLite partial index cannot.
-- SQLite serializes writes, so the check also protects concurrent inserts.
-- +goose StatementBegin
CREATE TRIGGER bot_token_insert BEFORE INSERT ON api_tokens
WHEN (SELECT account_type FROM users WHERE id=NEW.user_id)='bot' BEGIN
 SELECT CASE WHEN NEW.expires_at IS NOT NULL THEN RAISE(ABORT,'bot token cannot expire') END;
 SELECT CASE WHEN NEW.revoked_at IS NULL AND EXISTS(SELECT 1 FROM api_tokens WHERE user_id=NEW.user_id AND revoked_at IS NULL)
 THEN RAISE(ABORT,'bot already has a token') END;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER bot_token_update BEFORE UPDATE ON api_tokens BEGIN
 SELECT CASE WHEN NEW.user_id<>OLD.user_id THEN RAISE(ABORT,'token owner is immutable') END;
 SELECT CASE WHEN (SELECT account_type FROM users WHERE id=NEW.user_id)='bot'
 AND (NEW.expires_at IS NOT NULL OR (OLD.revoked_at IS NOT NULL AND NEW.revoked_at IS NULL)
 OR (NEW.revoked_at IS NULL AND EXISTS(SELECT 1 FROM api_tokens WHERE user_id=NEW.user_id AND id<>NEW.id AND revoked_at IS NULL)))
 THEN RAISE(ABORT,'invalid bot token update') END;
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER bot_no_web_session BEFORE INSERT ON web_sessions
WHEN (SELECT account_type FROM users WHERE id=NEW.user_id)='bot' BEGIN
 SELECT RAISE(ABORT,'bot cannot have a web session');
END;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE TRIGGER bot_no_web_session_update BEFORE UPDATE OF user_id ON web_sessions
WHEN (SELECT account_type FROM users WHERE id=NEW.user_id)='bot' BEGIN
 SELECT RAISE(ABORT,'bot cannot have a web session');
END;
-- +goose StatementEnd

-- +goose Down
-- Refuse destructive downgrade while bot identities/history exist.
CREATE TEMP TABLE bot_rollback_guard(value INTEGER CHECK(value=0));
INSERT INTO bot_rollback_guard SELECT count(*) FROM users WHERE account_type='bot';
DROP TABLE bot_rollback_guard;
DROP TRIGGER bot_no_web_session_update;
DROP TRIGGER bot_no_web_session;
DROP TRIGGER bot_token_update;
DROP TRIGGER bot_token_insert;
DROP TRIGGER users_account_update;
DROP TRIGGER users_account_insert;
ALTER TABLE users DROP COLUMN account_type;
