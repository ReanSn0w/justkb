package archive

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	appdb "github.com/ReanSn0w/justkb/internal/database"
)

type Filters struct {
	Type      string `json:"type,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
	Query     string `json:"q,omitempty"`
	From      string `json:"from,omitempty"`
	To        string `json:"to,omitempty"`
	State     string `json:"state,omitempty"`
	History   bool   `json:"history,omitempty"`
	Cursor    string `json:"cursor,omitempty"`
	Limit     int    `json:"limit,omitempty"`
	BatchID   string `json:"batch_id,omitempty"`
}
type Event struct {
	ID              string     `json:"id"`
	EntityType      string     `json:"entity_type"`
	EntityID        string     `json:"entity_id"`
	ProjectID       *string    `json:"project_id"`
	Action          string     `json:"action"`
	OccurredAt      *time.Time `json:"occurred_at"`
	Reason          string     `json:"reason"`
	ActorType       string     `json:"actor_type"`
	ActorName       *string    `json:"actor_name"`
	BatchID         *string    `json:"batch_id"`
	Title           string     `json:"title"`
	Number          *int64     `json:"number"`
	RuleDays        *int       `json:"rule_days"`
	IsArchived      bool       `json:"is_archived"`
	Version         int64      `json:"version"`
	ChildCount      int        `json:"child_count"`
	RestorableCards int        `json:"restorable_cards"`
	sortTime        string
}
type Page struct {
	Items      []Event `json:"items"`
	NextCursor string  `json:"next_cursor"`
}
type timelineCursor struct{ Time, ID string }

const eventSelect = `SELECT a.id,a.entity_type,a.entity_id,a.epic_id,a.action,a.occurred_at,a.reason,a.actor_type,u.display_name,a.batch_id,a.title,a.number,a.rule_days,
 CASE WHEN a.entity_type='card' THEN c.is_archived ELSE p.is_archived END,
 CASE WHEN a.entity_type='card' THEN c.version ELSE p.version END,
 CASE WHEN a.entity_type='project' THEN (SELECT count(*) FROM archive_events ch WHERE ch.batch_id=a.batch_id AND ch.entity_type='card' AND ch.action=a.action) ELSE 0 END,
 CASE WHEN a.entity_type='project' THEN (SELECT count(*) FROM cards rc WHERE rc.epic_id=p.id AND rc.is_archived AND rc.archive_batch_id=p.archive_batch_id) ELSE 0 END,
 COALESCE(strftime('%Y-%m-%dT%H:%M:%fZ',a.occurred_at),'')
 FROM archive_events a LEFT JOIN cards c ON a.entity_type='card' AND c.id=a.entity_id LEFT JOIN epics p ON a.entity_type='project' AND p.id=a.entity_id LEFT JOIN users u ON u.id=a.actor_user_id`

func scanEvent(row interface{ Scan(...any) error }) (Event, error) {
	var e Event
	err := row.Scan(&e.ID, &e.EntityType, &e.EntityID, &e.ProjectID, &e.Action, &e.OccurredAt, &e.Reason, &e.ActorType, &e.ActorName, &e.BatchID, &e.Title, &e.Number, &e.RuleDays, &e.IsArchived, &e.Version, &e.ChildCount, &e.RestorableCards, &e.sortTime)
	return e, err
}
func (s *Service) List(ctx context.Context, f Filters) (Page, error) {
	result := Page{Items: []Event{}}
	if f.Limit == 0 {
		f.Limit = 30
	}
	if f.Limit < 1 || f.Limit > 100 || (f.Type != "" && f.Type != "card" && f.Type != "project") || (f.State != "" && f.State != "archived" && f.State != "restored" && f.State != "all") {
		return result, ErrValidation
	}
	for _, date := range []string{f.From, f.To} {
		if date != "" {
			if _, err := time.Parse(time.RFC3339, date); err != nil {
				return result, ErrValidation
			}
		}
	}
	if f.From != "" && f.To != "" {
		from, _ := time.Parse(time.RFC3339, f.From)
		to, _ := time.Parse(time.RFC3339, f.To)
		if !from.Before(to) {
			return result, ErrValidation
		}
	}
	args := []any{}
	query := eventSelect + ` WHERE (c.id IS NOT NULL OR p.id IS NOT NULL)`
	add := func(clause string, value any) { args = append(args, value); query += fmt.Sprintf(clause, len(args)) }
	if f.Type != "" {
		add(` AND a.entity_type=$%d`, f.Type)
	}
	if f.ProjectID != "" {
		add(` AND a.epic_id=$%d`, f.ProjectID)
	}
	if f.Query != "" {
		add(` AND instr(lower(a.title||' '||COALESCE(a.number,'')),lower($%d))>0`, strings.TrimSpace(f.Query))
	}
	if f.From != "" {
		add(` AND julianday(a.occurred_at)>=julianday($%d)`, f.From)
	}
	if f.To != "" {
		add(` AND julianday(a.occurred_at)<julianday($%d)`, f.To)
	}
	if !f.History {
		query += ` AND a.action='archived' AND CASE WHEN a.entity_type='card' THEN c.is_archived ELSE p.is_archived END AND a.id=(SELECT last.id FROM archive_events last WHERE last.entity_type=a.entity_type AND last.entity_id=a.entity_id ORDER BY julianday(last.occurred_at) DESC,last.rowid DESC LIMIT 1)`
	}
	state := f.State
	if state == "" && !f.History {
		state = "archived"
	}
	if state == "archived" {
		query += ` AND CASE WHEN a.entity_type='card' THEN c.is_archived ELSE p.is_archived END`
	}
	if state == "restored" {
		query += ` AND NOT CASE WHEN a.entity_type='card' THEN c.is_archived ELSE p.is_archived END`
	}
	if f.BatchID != "" {
		add(` AND a.batch_id=$%d`, f.BatchID)
		query += ` AND a.entity_type='card'`
	} else if f.Type != "card" {
		query += ` AND (a.entity_type='project' OR a.batch_id IS NULL OR NOT EXISTS(SELECT 1 FROM archive_events parent JOIN epics ep ON ep.id=parent.entity_id WHERE parent.entity_type='project' AND parent.batch_id=a.batch_id AND parent.action=a.action`
		if !f.History {
			query += ` AND ep.is_archived AND ep.archive_batch_id=a.batch_id`
		}
		query += `))`
	}
	if f.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(f.Cursor)
		var cursor timelineCursor
		if err != nil || json.Unmarshal(raw, &cursor) != nil || cursor.ID == "" {
			return result, ErrValidation
		}
		args = append(args, cursor.Time, cursor.ID)
		query += fmt.Sprintf(` AND (COALESCE(strftime('%%Y-%%m-%%dT%%H:%%M:%%fZ',a.occurred_at),''),a.id)<($%d,$%d)`, len(args)-1, len(args))
	}
	args = append(args, f.Limit+1)
	query += fmt.Sprintf(` ORDER BY COALESCE(strftime('%%Y-%%m-%%dT%%H:%%M:%%fZ',a.occurred_at),'') DESC,a.id DESC LIMIT $%d`, len(args))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, e)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	if len(result.Items) > f.Limit {
		result.Items = result.Items[:f.Limit]
		last := result.Items[len(result.Items)-1]
		raw, _ := json.Marshal(timelineCursor{Time: last.sortTime, ID: last.ID})
		result.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return result, nil
}
func (s *Service) RestoreProject(ctx context.Context, actor Actor, id string, version int64, mode string) error {
	tx, err := appdb.BeginWrite(ctx, s.db)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := Project(ctx, tx, actor, id, version, false, mode, s.now()); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) RestoreCount(ctx context.Context, id string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM cards WHERE epic_id=e.id AND is_archived AND archive_batch_id=e.archive_batch_id) FROM epics e WHERE id=$1`, id).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	return count, err
}
