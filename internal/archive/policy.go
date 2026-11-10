package archive

import (
	"context"
	"database/sql"
	"time"

	appdb "github.com/ReanSn0w/justkb/internal/database"
)

type Service struct {
	db  *sql.DB
	now func() time.Time
}

func NewService(db *sql.DB) *Service { return &Service{db: db, now: time.Now} }

type Policy struct {
	Version         int64   `json:"version"`
	DoneColumnID    *string `json:"done_column_id"`
	CardsEnabled    bool    `json:"cards_enabled"`
	CardsDays       int     `json:"cards_days"`
	ProjectsEnabled bool    `json:"projects_enabled"`
	ProjectsDays    int     `json:"projects_days"`
}
type Preview struct {
	Cards    int `json:"cards"`
	Projects int `json:"projects"`
}
type querier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func policy(ctx context.Context, q querier) (Policy, error) {
	var p Policy
	err := q.QueryRowContext(ctx, `SELECT version,done_column_id,cards_enabled,cards_days,projects_enabled,projects_days FROM archive_policy WHERE singleton=1`).Scan(&p.Version, &p.DoneColumnID, &p.CardsEnabled, &p.CardsDays, &p.ProjectsEnabled, &p.ProjectsDays)
	return p, err
}
func (s *Service) Policy(ctx context.Context) (Policy, error) { return policy(ctx, s.db) }
func validatePolicy(ctx context.Context, q querier, p Policy) error {
	if p.CardsDays < 1 || p.CardsDays > 3650 || p.ProjectsDays < 1 || p.ProjectsDays > 3650 {
		return ErrValidation
	}
	if p.DoneColumnID == nil {
		if p.CardsEnabled || p.ProjectsEnabled {
			return ErrValidation
		}
		return nil
	}
	var n int
	if err := q.QueryRowContext(ctx, `SELECT count(*) FROM board_columns WHERE id=$1`, *p.DoneColumnID).Scan(&n); err != nil {
		return err
	}
	if n != 1 {
		return ErrValidation
	}
	return nil
}

// RefreshProjects runs in the caller's transaction, preserving the start of an
// uninterrupted completed interval. Empty IDs are harmless.
func RefreshProjects(ctx context.Context, tx *sql.Tx, now time.Time, projectIDs ...string) error {
	for _, id := range projectIDs {
		if id == "" {
			continue
		}
		_, err := tx.ExecContext(ctx, `UPDATE epics SET completed_since=CASE WHEN EXISTS(SELECT 1 FROM cards WHERE epic_id=epics.id) AND NOT EXISTS(SELECT 1 FROM cards WHERE epic_id=epics.id AND column_id IS NOT (SELECT done_column_id FROM archive_policy)) THEN COALESCE(completed_since,$2) ELSE NULL END WHERE id=$1`, id, stamp(now))
		if err != nil {
			return err
		}
	}
	return nil
}
func sameColumn(a, b *string) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
func (s *Service) SavePolicy(ctx context.Context, p Policy, expected int64) (Policy, error) {
	tx, err := appdb.BeginWrite(ctx, s.db)
	if err != nil {
		return Policy{}, err
	}
	defer tx.Rollback()
	current, err := policy(ctx, tx)
	if err != nil {
		return Policy{}, err
	}
	if expected != current.Version {
		return Policy{}, ErrVersionConflict
	}
	if err := validatePolicy(ctx, tx, p); err != nil {
		return Policy{}, err
	}
	_, err = tx.ExecContext(ctx, `UPDATE archive_policy SET version=version+1,done_column_id=$1,cards_enabled=$2,cards_days=$3,projects_enabled=$4,projects_days=$5 WHERE singleton=1`, p.DoneColumnID, p.CardsEnabled, p.CardsDays, p.ProjectsEnabled, p.ProjectsDays)
	if err != nil {
		return Policy{}, err
	}
	if !sameColumn(current.DoneColumnID, p.DoneColumnID) {
		now := stamp(s.now())
		if _, err := tx.ExecContext(ctx, `UPDATE cards SET column_entered_at=$1`, now); err != nil {
			return Policy{}, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE epics SET completed_since=CASE WHEN EXISTS(SELECT 1 FROM cards WHERE epic_id=epics.id) AND NOT EXISTS(SELECT 1 FROM cards WHERE epic_id=epics.id AND column_id IS NOT (SELECT done_column_id FROM archive_policy)) THEN $1 ELSE NULL END`, now); err != nil {
			return Policy{}, err
		}
	}
	p.Version = current.Version + 1
	if err := tx.Commit(); err != nil {
		return Policy{}, err
	}
	return p, nil
}
func (s *Service) Preview(ctx context.Context, p Policy) (Preview, error) {
	tx, err := appdb.BeginWrite(ctx, s.db)
	if err != nil {
		return Preview{}, err
	}
	defer tx.Rollback()
	if err := validatePolicy(ctx, tx, p); err != nil {
		return Preview{}, err
	}
	current, err := policy(ctx, tx)
	if err != nil {
		return Preview{}, err
	}
	// Changing the meaning of done starts fresh intervals: nothing is immediately due.
	if !sameColumn(current.DoneColumnID, p.DoneColumnID) {
		return Preview{}, nil
	}
	return preview(ctx, tx, p, s.now())
}

const projectDue = `NOT is_archived AND completed_since IS NOT NULL AND julianday(completed_since)<=julianday($1) AND EXISTS(SELECT 1 FROM cards WHERE epic_id=epics.id) AND NOT EXISTS(SELECT 1 FROM cards WHERE epic_id=epics.id AND column_id IS NOT $2)`
const cardDue = `NOT is_archived AND column_id=$2 AND column_entered_at IS NOT NULL AND julianday(column_entered_at)<=julianday($1) AND (epic_id IS NULL OR EXISTS(SELECT 1 FROM epics WHERE id=cards.epic_id AND NOT is_archived))`

func preview(ctx context.Context, q querier, p Policy, now time.Time) (Preview, error) {
	var result Preview
	if p.ProjectsEnabled {
		if err := q.QueryRowContext(ctx, `SELECT count(*) FROM epics WHERE `+projectDue, stamp(now.Add(-time.Duration(p.ProjectsDays)*24*time.Hour)), p.DoneColumnID).Scan(&result.Projects); err != nil {
			return result, err
		}
	}
	if p.CardsEnabled {
		if err := q.QueryRowContext(ctx, `SELECT count(*) FROM cards WHERE `+cardDue, stamp(now.Add(-time.Duration(p.CardsDays)*24*time.Hour)), p.DoneColumnID).Scan(&result.Cards); err != nil {
			return result, err
		}
	}
	return result, nil
}
