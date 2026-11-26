// Package updates owns the durable project event stream. Writers share the
// caller's transaction; links reflect membership at the time of the mutation.
package updates

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/ReanSn0w/justkb/internal/ids"
)

var (
	ErrNotFound = errors.New("project not found")
	ErrCursor   = errors.New("invalid updates cursor")
	ErrReset    = errors.New("updates cursor requires resynchronization")
	ErrLimit    = errors.New("invalid updates limit")
)

type Actor struct{ UserID, TokenID string }
type Event struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	OccurredAt   time.Time      `json:"occurred_at"`
	ProjectIDs   []string       `json:"project_ids"`
	EntityType   string         `json:"entity_type"`
	EntityID     string         `json:"entity_id"`
	CardID       string         `json:"card_id,omitempty"`
	CardNumber   int64          `json:"card_number,omitempty"`
	Version      int64          `json:"version,omitempty"`
	ActorType    string         `json:"actor_type"`
	ActorUserID  *string        `json:"actor_user_id"`
	ActorTokenID *string        `json:"actor_token_id"`
	Changes      map[string]any `json:"changes"`
}
type Page struct {
	Items      []Event `json:"items"`
	NextCursor string  `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}
type cursor struct {
	Version    int    `json:"v"`
	Generation string `json:"g"`
	Project    string `json:"p"`
	Sequence   int64  `json:"s"`
}
type Service struct{ db *sql.DB }

func NewService(db *sql.DB) *Service { return &Service{db} }

// Append never commits the caller's transaction. An event without a project has
// no subscriber and is deliberately omitted.
func Append(ctx context.Context, tx *sql.Tx, actor Actor, e Event) error {
	seen := map[string]bool{}
	projects := []string{}
	for _, p := range e.ProjectIDs {
		if p != "" && !seen[p] {
			seen[p] = true
			projects = append(projects, p)
		}
	}
	if len(projects) == 0 {
		return nil
	}
	sort.Strings(projects)
	e.ProjectIDs = projects
	e.ID = ids.New()
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	e.ActorType = "system"
	if actor.UserID != "" {
		e.ActorType = "user"
		e.ActorUserID = &actor.UserID
	}
	if actor.TokenID != "" {
		e.ActorTokenID = &actor.TokenID
	}
	raw, err := json.Marshal(e)
	if err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO project_updates(id,payload) VALUES(?,?)`, e.ID, string(raw))
	if err != nil {
		return err
	}
	seq, err := result.LastInsertId()
	if err != nil {
		return err
	}
	for _, p := range projects {
		if _, err = tx.ExecContext(ctx, `INSERT INTO project_update_links(project_id,event_sequence) VALUES(?,?)`, p, seq); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) List(ctx context.Context, project, token string, limit int) (Page, error) {
	if limit == 0 {
		limit = 100
	}
	if limit < 1 || limit > 500 {
		return Page{}, ErrLimit
	}
	// A short transaction keeps existence, high-water mark and page consistent.
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Page{}, err
	}
	defer tx.Rollback()
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM epics WHERE id=?)`, project).Scan(&exists); err != nil {
		return Page{}, err
	}
	if !exists {
		return Page{}, ErrNotFound
	}
	c := cursor{Version: 1, Project: project}
	var high int64
	if err = tx.QueryRowContext(ctx, `SELECT generation,(SELECT COALESCE(MAX(sequence),0) FROM project_updates) FROM project_update_generation WHERE singleton=1`).Scan(&c.Generation, &high); err != nil {
		return Page{}, err
	}
	if token != "" && token != "now" {
		var decoded cursor
		raw, e := base64.RawURLEncoding.DecodeString(token)
		if e != nil || json.Unmarshal(raw, &decoded) != nil || decoded.Version != 1 || decoded.Project != project || decoded.Sequence < 0 || decoded.Generation == "" {
			return Page{}, ErrCursor
		}
		if decoded.Generation != c.Generation || decoded.Sequence > high {
			return Page{}, ErrReset
		}
		c = decoded
	}
	page := Page{Items: []Event{}}
	if token != "now" {
		rows, e := tx.QueryContext(ctx, `SELECT u.sequence,u.payload FROM project_update_links l JOIN project_updates u ON u.sequence=l.event_sequence WHERE l.project_id=? AND l.event_sequence>? AND l.event_sequence<=? ORDER BY l.event_sequence LIMIT ?`, project, c.Sequence, high, limit+1)
		if e != nil {
			return Page{}, e
		}
		for rows.Next() {
			var seq int64
			var raw []byte
			var event Event
			if e = rows.Scan(&seq, &raw); e != nil {
				rows.Close()
				return Page{}, e
			}
			if len(page.Items) == limit {
				page.HasMore = true
				break
			}
			if e = json.Unmarshal(raw, &event); e != nil {
				rows.Close()
				return Page{}, e
			}
			page.Items = append(page.Items, event)
			c.Sequence = seq
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return Page{}, e
		}
	}
	if !page.HasMore {
		c.Sequence = high
	}
	raw, _ := json.Marshal(c)
	page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	if err = tx.Commit(); err != nil {
		return Page{}, err
	}
	return page, nil
}

// Related captures project ownership inside the mutation transaction.
func Related(ctx context.Context, tx *sql.Tx, actor Actor, kind, id, cardID, projectID string, changes map[string]any) error {
	e := Event{Type: kind, EntityID: id, CardID: cardID, Changes: changes}
	switch kind {
	case "comment.created":
		e.EntityType = "comment"
	default:
		e.EntityType = "attachment"
	}
	if cardID != "" {
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(epic_id,''),number FROM cards WHERE id=?`, cardID).Scan(&projectID, &e.CardNumber); err != nil {
			return err
		}
	}
	e.ProjectIDs = []string{projectID}
	return Append(ctx, tx, actor, e)
}
