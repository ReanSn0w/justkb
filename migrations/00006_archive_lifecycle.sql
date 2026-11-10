-- +goose Up
CREATE TABLE archive_policy (
 singleton INTEGER PRIMARY KEY CHECK(singleton=1),
 version INTEGER NOT NULL DEFAULT 1,
 done_column_id TEXT REFERENCES board_columns(id) ON DELETE RESTRICT,
 cards_enabled INTEGER NOT NULL DEFAULT 0 CHECK(cards_enabled IN (0,1)),
 cards_days INTEGER NOT NULL DEFAULT 3 CHECK(cards_days BETWEEN 1 AND 3650),
 projects_enabled INTEGER NOT NULL DEFAULT 0 CHECK(projects_enabled IN (0,1)),
 projects_days INTEGER NOT NULL DEFAULT 3 CHECK(projects_days BETWEEN 1 AND 3650)
);
INSERT INTO archive_policy(singleton,done_column_id) VALUES(1,(SELECT id FROM board_columns ORDER BY position DESC LIMIT 1));
ALTER TABLE cards ADD COLUMN column_entered_at DATETIME;
ALTER TABLE cards ADD COLUMN archive_batch_id TEXT;
UPDATE cards SET column_entered_at=CURRENT_TIMESTAMP;
ALTER TABLE epics ADD COLUMN version INTEGER NOT NULL DEFAULT 1 CHECK(version>0);
ALTER TABLE epics ADD COLUMN completed_since DATETIME;
ALTER TABLE epics ADD COLUMN archive_batch_id TEXT;
UPDATE epics SET completed_since=CURRENT_TIMESTAMP WHERE EXISTS(SELECT 1 FROM cards WHERE epic_id=epics.id) AND NOT EXISTS(SELECT 1 FROM cards WHERE epic_id=epics.id AND column_id IS NOT (SELECT done_column_id FROM archive_policy));
CREATE INDEX cards_archive_due_idx ON cards(is_archived,column_id,column_entered_at);
CREATE INDEX epics_archive_due_idx ON epics(is_archived,completed_since);
CREATE TABLE archive_events (
 id TEXT PRIMARY KEY,
 entity_type TEXT NOT NULL CHECK(entity_type IN ('card','project')),
 entity_id TEXT NOT NULL,
 epic_id TEXT,
 action TEXT NOT NULL CHECK(action IN ('archived','restored')),
 occurred_at DATETIME,
 reason TEXT NOT NULL CHECK(reason IN ('manual','automatic','legacy')),
 actor_type TEXT NOT NULL CHECK(actor_type IN ('user','system','unknown')),
 actor_user_id TEXT REFERENCES users(id) ON DELETE RESTRICT,
 batch_id TEXT,
 title TEXT NOT NULL,
 number INTEGER,
 rule_days INTEGER
);
CREATE INDEX archive_events_timeline_idx ON archive_events(occurred_at DESC,id DESC);
CREATE INDEX archive_events_entity_idx ON archive_events(entity_type,entity_id,occurred_at DESC,id DESC);
CREATE INDEX archive_events_batch_idx ON archive_events(batch_id);
CREATE TABLE card_activity_new (
 id TEXT PRIMARY KEY,
 card_id TEXT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
 actor_user_id TEXT REFERENCES users(id) ON DELETE RESTRICT,
 actor_token_id TEXT REFERENCES api_tokens(id) ON DELETE SET NULL,
 action TEXT NOT NULL CHECK(trim(action)<>''),
 changes TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(changes)),
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
 actor_type TEXT NOT NULL DEFAULT 'user' CHECK(actor_type IN ('user','system')),
 CHECK((actor_type='user' AND actor_user_id IS NOT NULL) OR (actor_type='system' AND actor_user_id IS NULL AND actor_token_id IS NULL))
);
INSERT INTO card_activity_new(id,card_id,actor_user_id,actor_token_id,action,changes,created_at) SELECT id,card_id,actor_user_id,actor_token_id,action,changes,created_at FROM card_activity;
DROP TABLE card_activity;
ALTER TABLE card_activity_new RENAME TO card_activity;
CREATE INDEX card_activity_card_idx ON card_activity(card_id,created_at DESC,id DESC);
CREATE INDEX card_activity_moved_created_idx ON card_activity(created_at DESC,id DESC) WHERE action='moved';
INSERT INTO archive_events(id,entity_type,entity_id,epic_id,action,occurred_at,reason,actor_type,actor_user_id,title,number)
 SELECT a.id,'card',c.id,c.epic_id,a.action,a.created_at,'legacy','user',a.actor_user_id,c.title,c.number FROM card_activity a JOIN cards c ON c.id=a.card_id WHERE a.action IN ('archived','restored');
INSERT INTO archive_events(id,entity_type,entity_id,epic_id,action,reason,actor_type,title,number)
 SELECT 'legacy-card-'||id,'card',id,epic_id,'archived','legacy','unknown',title,number FROM cards c WHERE is_archived AND NOT EXISTS(SELECT 1 FROM archive_events e WHERE e.entity_type='card' AND e.entity_id=c.id);
INSERT INTO archive_events(id,entity_type,entity_id,epic_id,action,reason,actor_type,title)
 SELECT 'legacy-project-'||id,'project',id,id,'archived','legacy','unknown',name FROM epics WHERE is_archived;

-- +goose Down
CREATE TEMP TABLE archive_rollback_guard(value INTEGER CHECK(value=0));
INSERT INTO archive_rollback_guard SELECT count(*) FROM archive_events;
INSERT INTO archive_rollback_guard SELECT count(*) FROM card_activity WHERE actor_type='system';
INSERT INTO archive_rollback_guard SELECT count(*) FROM archive_policy WHERE version<>1 OR cards_enabled OR projects_enabled;
DROP TABLE archive_rollback_guard;
DROP TABLE archive_events;
DROP TABLE archive_policy;
DROP INDEX cards_archive_due_idx;
DROP INDEX epics_archive_due_idx;
ALTER TABLE cards DROP COLUMN column_entered_at;
ALTER TABLE cards DROP COLUMN archive_batch_id;
ALTER TABLE epics DROP COLUMN version;
ALTER TABLE epics DROP COLUMN completed_since;
ALTER TABLE epics DROP COLUMN archive_batch_id;
CREATE TABLE card_activity_old (
 id TEXT PRIMARY KEY,
 card_id TEXT NOT NULL REFERENCES cards(id) ON DELETE CASCADE,
 actor_user_id TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
 actor_token_id TEXT REFERENCES api_tokens(id) ON DELETE SET NULL,
 action TEXT NOT NULL CHECK(trim(action)<>''),
 changes TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(changes)),
 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO card_activity_old SELECT id,card_id,actor_user_id,actor_token_id,action,changes,created_at FROM card_activity;
DROP TABLE card_activity;
ALTER TABLE card_activity_old RENAME TO card_activity;
CREATE INDEX card_activity_card_idx ON card_activity(card_id,created_at DESC,id DESC);
CREATE INDEX card_activity_moved_created_idx ON card_activity(created_at DESC,id DESC) WHERE action='moved';
