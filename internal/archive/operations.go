// Package archive owns transactional archive transitions shared by every transport
// and the background worker. Callers own the write transaction.
package archive

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/ReanSn0w/justkb/internal/ids"
)

var (
	ErrNotFound        = errors.New("archive object not found")
	ErrVersionConflict = errors.New("archive version conflict")
	ErrArchivedProject = errors.New("restore the project first")
	ErrReadOnly        = errors.New("restore the archived object before editing")
	ErrNameConflict    = errors.New("active project name already exists")
	ErrValidation      = errors.New("invalid archive operation")
)

type Actor struct {
	UserID, TokenID string
	Automatic       bool
	RuleDays        int
}

func (a Actor) values() (string, string, any, any, error) {
	if a.Automatic {
		if a.UserID != "" || a.TokenID != "" {
			return "", "", nil, nil, ErrValidation
		}
		return "system", "automatic", nil, nil, nil
	}
	if a.UserID == "" {
		return "", "", nil, nil, ErrValidation
	}
	var token any
	if a.TokenID != "" {
		token = a.TokenID
	}
	return "user", "manual", a.UserID, token, nil
}

func event(ctx context.Context, tx *sql.Tx, actor Actor, kind, id, project, action, batch, title string, number any, now time.Time) error {
	actorType, reason, user, _, err := actor.values()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO archive_events(id,entity_type,entity_id,epic_id,action,occurred_at,reason,actor_type,actor_user_id,batch_id,title,number,rule_days) VALUES($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,$9,NULLIF($10,''),$11,$12,$13)`, ids.New(), kind, id, project, action, stamp(now), reason, actorType, user, batch, title, number, actor.RuleDays)
	return err
}

func Card(ctx context.Context, tx *sql.Tx, actor Actor, id string, version int64, archived bool, batch string, now time.Time) error {
	var current bool
	var actual, number int64
	var title, column string
	var epic sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT version,is_archived,title,number,column_id,epic_id FROM cards WHERE id=$1`, id).Scan(&actual, &current, &title, &number, &column, &epic)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if version != actual {
		return ErrVersionConflict
	}
	if current == archived {
		return nil
	}
	if !archived && epic.Valid {
		var parentArchived bool
		if err := tx.QueryRowContext(ctx, `SELECT is_archived FROM epics WHERE id=$1`, epic.String).Scan(&parentArchived); err != nil {
			return err
		}
		if parentArchived {
			return ErrArchivedProject
		}
	}
	actorType, _, user, token, err := actor.values()
	if err != nil {
		return err
	}
	action := "archived"
	if !archived {
		action = "restored"
	}
	_, err = tx.ExecContext(ctx, `UPDATE cards SET is_archived=$2,version=version+1,updated_at=$3,archive_batch_id=CASE WHEN $2 THEN NULLIF($4,'') ELSE NULL END,column_entered_at=CASE WHEN $2 THEN column_entered_at ELSE $3 END,position=CASE WHEN $2 THEN position ELSE COALESCE((SELECT min(c.position)-1000 FROM cards c WHERE c.column_id=cards.column_id AND NOT c.is_archived),1000) END WHERE id=$1`, id, archived, stamp(now), batch)
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]any{"from_version": version, "batch_id": batch})
	if _, err = tx.ExecContext(ctx, `INSERT INTO card_activity(id,card_id,actor_user_id,actor_token_id,actor_type,action,changes,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, ids.New(), id, user, token, actorType, action, string(payload), stamp(now)); err != nil {
		return err
	}
	if err = event(ctx, tx, actor, "card", id, epic.String, action, batch, title, number, now); err != nil {
		return err
	}
	return RefreshProjects(ctx, tx, now, epic.String)
}

// Project restores only members of the most recent archive batch. A subsequent
// independent card transition clears/replaces that membership.
func Project(ctx context.Context, tx *sql.Tx, actor Actor, id string, version int64, archived bool, mode string, now time.Time) error {
	if !archived && mode != "project_only" && mode != "with_cards" {
		return ErrValidation
	}
	var current bool
	var actual int64
	var title string
	var previous sql.NullString
	err := tx.QueryRowContext(ctx, `SELECT version,is_archived,name,archive_batch_id FROM epics WHERE id=$1`, id).Scan(&actual, &current, &title, &previous)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if version != actual {
		return ErrVersionConflict
	}
	if current == archived {
		return nil
	}
	if !archived {
		var duplicate bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM epics WHERE id<>$1 AND NOT is_archived AND lower(name)=lower($2))`, id, title).Scan(&duplicate); err != nil {
			return err
		}
		if duplicate {
			return ErrNameConflict
		}
	}
	batch := ids.New()
	action := "archived"
	if !archived {
		action = "restored"
	}
	if _, err := tx.ExecContext(ctx, `UPDATE epics SET is_archived=$2,version=version+1,updated_at=$3,archive_batch_id=$4,completed_since=CASE WHEN NOT $2 AND completed_since IS NOT NULL THEN $3 ELSE completed_since END WHERE id=$1`, id, archived, stamp(now), batch); err != nil {
		return err
	}
	if archived || mode == "with_cards" {
		rows, err := tx.QueryContext(ctx, `SELECT id,version FROM cards WHERE epic_id=$1 AND (($2 AND NOT is_archived) OR (NOT $2 AND is_archived AND archive_batch_id=$3)) ORDER BY position,id`, id, archived, previous)
		if err != nil {
			return err
		}
		type candidate struct {
			id      string
			version int64
		}
		items := []candidate{}
		for rows.Next() {
			var c candidate
			if err = rows.Scan(&c.id, &c.version); err != nil {
				rows.Close()
				return err
			}
			items = append(items, c)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, c := range items {
			if err := Card(ctx, tx, actor, c.id, c.version, archived, batch, now); err != nil {
				return err
			}
		}
	}
	return event(ctx, tx, actor, "project", id, id, action, batch, title, nil, now)
}

func stamp(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func EnsureEditable(ctx context.Context, tx *sql.Tx, cardID, epicID string) error {
	var archived bool
	query := `SELECT is_archived FROM cards WHERE id=$1`
	id := cardID
	if cardID == "" {
		query = `SELECT is_archived FROM epics WHERE id=$1`
		id = epicID
	}
	err := tx.QueryRowContext(ctx, query, id).Scan(&archived)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if archived {
		return ErrReadOnly
	}
	return nil
}
