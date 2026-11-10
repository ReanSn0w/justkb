package archive

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	appdb "github.com/ReanSn0w/justkb/internal/database"
	"github.com/ReanSn0w/justkb/internal/maintenance"
)

type PassResult struct{ Projects, Cards int }

// RunOnce takes no candidate or policy across maintenance boundaries. Each
// transaction selects and rechecks one current candidate under SQLite's writer
// lock; a project and all its cards remain one atomic unit.
func (s *Service) RunOnce(ctx context.Context, gate *maintenance.Gate) (PassResult, error) {
	var result PassResult
	for _, project := range []bool{true, false} {
		for {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			changed := false
			err := gate.Shared(func() error {
				tx, err := appdb.BeginWrite(ctx, s.db)
				if err != nil {
					return err
				}
				defer tx.Rollback()
				p, err := policy(ctx, tx)
				if err != nil {
					return err
				}
				days := p.CardsDays
				enabled := p.CardsEnabled
				query := `SELECT id,version FROM cards WHERE ` + cardDue + ` ORDER BY column_entered_at,id LIMIT 1`
				if project {
					days = p.ProjectsDays
					enabled = p.ProjectsEnabled
					query = `SELECT id,version FROM epics WHERE ` + projectDue + ` ORDER BY completed_since,id LIMIT 1`
				}
				if !enabled || p.DoneColumnID == nil {
					return nil
				}
				now := s.now()
				var id string
				var version int64
				err = tx.QueryRowContext(ctx, query, stamp(now.Add(-time.Duration(days)*24*time.Hour)), p.DoneColumnID).Scan(&id, &version)
				if errors.Is(err, sql.ErrNoRows) {
					return nil
				}
				if err != nil {
					return err
				}
				actor := Actor{Automatic: true, RuleDays: days}
				if project {
					err = Project(ctx, tx, actor, id, version, true, "", now)
				} else {
					err = Card(ctx, tx, actor, id, version, true, "", now)
				}
				if err != nil {
					return err
				}
				if err = tx.Commit(); err != nil {
					return err
				}
				changed = true
				return nil
			})
			if err != nil {
				return result, err
			}
			if !changed {
				break
			}
			if project {
				result.Projects++
			} else {
				result.Cards++
			}
		}
	}
	return result, nil
}
func (s *Service) Run(ctx context.Context, gate *maintenance.Gate, logger *slog.Logger) {
	pass := func() {
		if ctx.Err() != nil {
			return
		}
		result, err := s.RunOnce(ctx, gate)
		if err != nil {
			if ctx.Err() == nil {
				logger.Error("archive pass skipped or failed; explicit schema migration may be required", "error", err)
			}
			return
		}
		logger.Info("archive pass completed", "projects", result.Projects, "cards", result.Cards)
	}
	pass()
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pass()
		}
	}
}
