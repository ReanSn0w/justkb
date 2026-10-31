package cards

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrNotFound        = errors.New("card not found")
	ErrValidation      = errors.New("validation failed")
	ErrVersionConflict = errors.New("card version conflict")
)

type Attachment struct {
	ID           string    `json:"id"`
	OriginalName string    `json:"original_name"`
	MediaType    string    `json:"media_type"`
	Size         int64     `json:"size"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
}

type Card struct {
	ID          string       `json:"id"`
	Number      int64        `json:"number"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	ColumnID    string       `json:"column_id"`
	EpicID      *string      `json:"epic_id"`
	GroupID     *string      `json:"group_id"`
	AssigneeID  *string      `json:"assignee_id"`
	Position    string       `json:"position"`
	DueAt       *time.Time   `json:"due_at"`
	Version     int64        `json:"version"`
	IsArchived  bool         `json:"is_archived"`
	CreatedBy   string       `json:"created_by"`
	TagIDs      []string     `json:"tag_ids"`
	Attachments []Attachment `json:"attachments"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

type Activity struct {
	ID           string         `json:"id"`
	CardID       string         `json:"card_id"`
	ActorUserID  string         `json:"actor_user_id"`
	ActorTokenID *string        `json:"actor_token_id"`
	Action       string         `json:"action"`
	Changes      map[string]any `json:"changes"`
	CreatedAt    time.Time      `json:"created_at"`
}

type Actor struct {
	UserID  string
	TokenID string
}

type CreateInput struct {
	Title       string
	Description string
	ColumnID    string
	EpicID      string
	GroupID     string
	AssigneeID  string
	TagIDs      []string
	DueAt       *time.Time
}

type UpdateInput struct {
	ExpectedVersion int64
	Title           *string
	Description     *string
	EpicID          *string
	GroupID         *string
	AssigneeID      *string
	TagIDs          *[]string
	DueAt           **time.Time
}

type MoveInput struct {
	ExpectedVersion int64
	TargetColumnID  string
	BeforeCardID    string
	AfterCardID     string
}

type Filters struct {
	EpicID        string
	GroupID       string
	AssigneeID    string
	CurrentUserID string
	TagIDs        []string
	ColumnID      string
	Query         string
	Archived      bool
	Limit         int
	Cursor        string
}

type Page struct {
	Items      []Card `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

type Service struct{ db *sql.DB }

func NewService(db *sql.DB) *Service { return &Service{db: db} }

func (s *Service) Create(ctx context.Context, actor Actor, input CreateInput) (Card, error) {
	if actor.UserID == "" || strings.TrimSpace(input.Title) == "" || input.ColumnID == "" {
		return Card{}, ErrValidation
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Card{}, err
	}
	defer tx.Rollback()
	if err := validateEpic(ctx, tx, input.EpicID); err != nil {
		return Card{}, err
	}
	var columnExists bool
	if err := tx.QueryRowContext(ctx, `SELECT true FROM board_columns WHERE id=$1 FOR UPDATE`, input.ColumnID).Scan(&columnExists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Card{}, ErrValidation
		}
		return Card{}, err
	}
	var position string
	if err := tx.QueryRowContext(ctx, `SELECT (COALESCE(max(position), 0) + 1000)::text FROM cards WHERE column_id=$1 AND NOT is_archived`, input.ColumnID).Scan(&position); err != nil {
		return Card{}, err
	}
	var card Card
	err = tx.QueryRowContext(ctx, `
		INSERT INTO cards (title,description,column_id,epic_id,group_id,assignee_id,position,due_at,created_by)
		VALUES ($1,$2,$3,NULLIF($4,'')::uuid,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,$7::numeric,$8,$9)
		RETURNING id,number,title,description,column_id,epic_id,group_id,assignee_id,position::text,due_at,version,is_archived,created_by,created_at,updated_at`,
		strings.TrimSpace(input.Title), input.Description, input.ColumnID, input.EpicID, input.GroupID, input.AssigneeID, position, input.DueAt, actor.UserID,
	).Scan(&card.ID, &card.Number, &card.Title, &card.Description, &card.ColumnID, &card.EpicID, &card.GroupID, &card.AssigneeID, &card.Position, &card.DueAt, &card.Version, &card.IsArchived, &card.CreatedBy, &card.CreatedAt, &card.UpdatedAt)
	if err != nil {
		return Card{}, err
	}
	if err := replaceTags(ctx, tx, card.ID, input.TagIDs); err != nil {
		return Card{}, err
	}
	if err := recordActivity(ctx, tx, actor, card.ID, "created", map[string]any{"version": card.Version}); err != nil {
		return Card{}, err
	}
	if err := tx.Commit(); err != nil {
		return Card{}, err
	}
	return s.Get(ctx, card.ID)
}

func (s *Service) Get(ctx context.Context, idOrNumber string) (Card, error) {
	query := cardSelect + ` WHERE c.id::text=$1`
	argument := idOrNumber
	if strings.HasPrefix(strings.ToUpper(idOrNumber), "JKB-") {
		query = cardSelect + ` WHERE c.number=$1`
		argument = strings.TrimPrefix(strings.ToUpper(idOrNumber), "JKB-")
	}
	row := s.db.QueryRowContext(ctx, query, argument)
	return scanCard(row)
}

func (s *Service) List(ctx context.Context, filters Filters) (Page, error) {
	if filters.Limit <= 0 {
		filters.Limit = 50
	}
	if filters.Limit > 100 {
		return Page{}, ErrValidation
	}
	query := cardSelect + ` JOIN board_columns ordering_column ON ordering_column.id=c.column_id WHERE c.is_archived=$1`
	args := []any{filters.Archived}
	add := func(clause string, value any) {
		args = append(args, value)
		query += fmt.Sprintf(clause, len(args))
	}
	if filters.EpicID == "none" {
		query += ` AND c.epic_id IS NULL`
	} else if filters.EpicID != "" {
		add(` AND c.epic_id=$%d`, filters.EpicID)
	}
	if filters.GroupID == "none" {
		query += ` AND c.group_id IS NULL`
	} else if filters.GroupID != "" {
		add(` AND c.group_id=$%d`, filters.GroupID)
	}
	if filters.AssigneeID == "none" {
		query += ` AND c.assignee_id IS NULL`
	} else if filters.AssigneeID == "me" {
		if filters.CurrentUserID == "" {
			return Page{}, ErrValidation
		}
		add(` AND c.assignee_id=$%d`, filters.CurrentUserID)
	} else if filters.AssigneeID != "" {
		add(` AND c.assignee_id=$%d`, filters.AssigneeID)
	}
	if filters.ColumnID != "" {
		add(` AND c.column_id=$%d`, filters.ColumnID)
	}
	if strings.TrimSpace(filters.Query) != "" {
		args = append(args, strings.TrimSpace(filters.Query))
		index := len(args)
		query += fmt.Sprintf(` AND (c.number::text=$%d OR to_tsvector('simple',c.title||' '||c.description) @@ plainto_tsquery('simple',$%d))`, index, index)
	}
	if len(filters.TagIDs) > 0 {
		args = append(args, filters.TagIDs)
		index := len(args)
		query += fmt.Sprintf(` AND (SELECT count(DISTINCT ct.tag_id) FROM card_tags ct WHERE ct.card_id=c.id AND ct.tag_id=ANY($%d::uuid[]))=$%d`, index, index+1)
		args = append(args, len(filters.TagIDs))
	}
	if filters.Cursor != "" {
		cursor, err := decodeCursor(filters.Cursor)
		if err != nil {
			return Page{}, ErrValidation
		}
		args = append(args, cursor.ColumnPosition, cursor.Position, cursor.ID)
		query += fmt.Sprintf(` AND (ordering_column.position,c.position,c.id) > ($%d,$%d::numeric,$%d::uuid)`, len(args)-2, len(args)-1, len(args))
	}
	args = append(args, filters.Limit+1)
	query += fmt.Sprintf(` ORDER BY ordering_column.position,c.position,c.id LIMIT $%d`, len(args))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	items := make([]Card, 0, filters.Limit+1)
	for rows.Next() {
		card, err := scanCard(rows)
		if err != nil {
			return Page{}, err
		}
		items = append(items, card)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	page := Page{Items: items}
	if len(items) > filters.Limit {
		last := items[filters.Limit-1]
		var columnPosition int
		if err := s.db.QueryRowContext(ctx, `SELECT position FROM board_columns WHERE id=$1`, last.ColumnID).Scan(&columnPosition); err != nil {
			return Page{}, err
		}
		page.Items = items[:filters.Limit]
		page.NextCursor = encodeCursor(listCursor{ColumnPosition: columnPosition, Position: last.Position, ID: last.ID})
	}
	return page, nil
}

func (s *Service) Update(ctx context.Context, actor Actor, cardID string, input UpdateInput) (Card, error) {
	if input.ExpectedVersion <= 0 {
		return Card{}, ErrValidation
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Card{}, err
	}
	defer tx.Rollback()
	current, err := getForUpdate(ctx, tx, cardID)
	if err != nil {
		return Card{}, err
	}
	if current.Version != input.ExpectedVersion {
		return Card{}, ErrVersionConflict
	}
	if input.Title != nil && strings.TrimSpace(*input.Title) == "" {
		return Card{}, ErrValidation
	}
	if input.EpicID != nil {
		if err := validateEpic(ctx, tx, *input.EpicID); err != nil {
			return Card{}, err
		}
	}
	title, description := current.Title, current.Description
	epicID, groupID, assigneeID := value(current.EpicID), value(current.GroupID), value(current.AssigneeID)
	dueAt := current.DueAt
	if input.Title != nil {
		title = strings.TrimSpace(*input.Title)
	}
	if input.Description != nil {
		description = *input.Description
	}
	if input.EpicID != nil {
		epicID = *input.EpicID
	}
	if input.GroupID != nil {
		groupID = *input.GroupID
	}
	if input.AssigneeID != nil {
		assigneeID = *input.AssigneeID
	}
	if input.DueAt != nil {
		dueAt = *input.DueAt
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE cards SET title=$3,description=$4,epic_id=NULLIF($5,'')::uuid,group_id=NULLIF($6,'')::uuid,
		assignee_id=NULLIF($7,'')::uuid,due_at=$8,version=version+1,updated_at=now()
		WHERE id=$1 AND version=$2`, cardID, input.ExpectedVersion, title, description, epicID, groupID, assigneeID, dueAt)
	if err != nil {
		return Card{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return Card{}, ErrVersionConflict
	}
	if input.TagIDs != nil {
		if err := replaceTags(ctx, tx, cardID, *input.TagIDs); err != nil {
			return Card{}, err
		}
	}
	if err := recordActivity(ctx, tx, actor, cardID, "updated", map[string]any{"from_version": input.ExpectedVersion}); err != nil {
		return Card{}, err
	}
	if err := tx.Commit(); err != nil {
		return Card{}, err
	}
	return s.Get(ctx, cardID)
}

func (s *Service) Move(ctx context.Context, actor Actor, cardID string, input MoveInput) (Card, error) {
	if input.ExpectedVersion <= 0 || input.TargetColumnID == "" || (input.BeforeCardID != "" && input.AfterCardID != "") {
		return Card{}, ErrValidation
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Card{}, err
	}
	defer tx.Rollback()
	current, err := getForUpdate(ctx, tx, cardID)
	if err != nil {
		return Card{}, err
	}
	if current.Version != input.ExpectedVersion {
		return Card{}, ErrVersionConflict
	}
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT true FROM board_columns WHERE id=$1 FOR UPDATE`, input.TargetColumnID).Scan(&exists); err != nil {
		return Card{}, ErrValidation
	}
	position, err := movePosition(ctx, tx, input.TargetColumnID, cardID, input.BeforeCardID, input.AfterCardID)
	if err != nil {
		return Card{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE cards SET column_id=$3,position=$4::numeric,version=version+1,updated_at=now() WHERE id=$1 AND version=$2`, cardID, input.ExpectedVersion, input.TargetColumnID, position)
	if err != nil {
		return Card{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return Card{}, ErrVersionConflict
	}
	if err := recordActivity(ctx, tx, actor, cardID, "moved", map[string]any{"from_column_id": current.ColumnID, "to_column_id": input.TargetColumnID}); err != nil {
		return Card{}, err
	}
	if err := tx.Commit(); err != nil {
		return Card{}, err
	}
	return s.Get(ctx, cardID)
}

func (s *Service) SetArchived(ctx context.Context, actor Actor, cardID string, expectedVersion int64, archived bool) (Card, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Card{}, err
	}
	defer tx.Rollback()
	current, err := getForUpdate(ctx, tx, cardID)
	if err != nil {
		return Card{}, err
	}
	if current.Version != expectedVersion {
		return Card{}, ErrVersionConflict
	}
	result, err := tx.ExecContext(ctx, `UPDATE cards SET is_archived=$3,version=version+1,updated_at=now() WHERE id=$1 AND version=$2`, cardID, expectedVersion, archived)
	if err != nil {
		return Card{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return Card{}, ErrVersionConflict
	}
	action := "archived"
	if !archived {
		action = "restored"
	}
	if err := recordActivity(ctx, tx, actor, cardID, action, map[string]any{"from_version": expectedVersion}); err != nil {
		return Card{}, err
	}
	if err := tx.Commit(); err != nil {
		return Card{}, err
	}
	return s.Get(ctx, cardID)
}

func (s *Service) Activity(ctx context.Context, cardID string, limit int) ([]Activity, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		return nil, ErrValidation
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id,card_id,actor_user_id,actor_token_id,action,changes,created_at FROM card_activity WHERE card_id=$1 ORDER BY created_at DESC,id DESC LIMIT $2`, cardID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Activity, 0)
	for rows.Next() {
		var item Activity
		var changes []byte
		if err := rows.Scan(&item.ID, &item.CardID, &item.ActorUserID, &item.ActorTokenID, &item.Action, &changes, &item.CreatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(changes, &item.Changes); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

const cardSelect = `
	SELECT c.id,c.number,c.title,c.description,c.column_id,c.epic_id,c.group_id,c.assignee_id,c.position::text,c.due_at,c.version,c.is_archived,c.created_by,c.created_at,c.updated_at,
	COALESCE((SELECT json_agg(ct.tag_id ORDER BY ct.tag_id) FROM card_tags ct WHERE ct.card_id=c.id),'[]'::json),
	COALESCE((SELECT json_agg(json_build_object('id',a.id,'original_name',a.original_name,'media_type',a.media_type,'size',a.size,'created_by',a.created_by,'created_at',a.created_at) ORDER BY a.created_at,a.id) FROM attachments a WHERE a.card_id=c.id),'[]'::json)
	FROM cards c`

type scanner interface{ Scan(...any) error }

func scanCard(row scanner) (Card, error) {
	var card Card
	var tags, attachments []byte
	err := row.Scan(&card.ID, &card.Number, &card.Title, &card.Description, &card.ColumnID, &card.EpicID, &card.GroupID, &card.AssigneeID, &card.Position, &card.DueAt, &card.Version, &card.IsArchived, &card.CreatedBy, &card.CreatedAt, &card.UpdatedAt, &tags, &attachments)
	if errors.Is(err, sql.ErrNoRows) {
		return Card{}, ErrNotFound
	}
	if err != nil {
		return Card{}, err
	}
	if err := json.Unmarshal(tags, &card.TagIDs); err != nil {
		return Card{}, err
	}
	if err := json.Unmarshal(attachments, &card.Attachments); err != nil {
		return Card{}, err
	}
	return card, nil
}

func getForUpdate(ctx context.Context, tx *sql.Tx, cardID string) (Card, error) {
	row := tx.QueryRowContext(ctx, cardSelect+` WHERE c.id=$1 FOR UPDATE`, cardID)
	return scanCard(row)
}

func validateEpic(ctx context.Context, tx *sql.Tx, epicID string) error {
	if epicID == "" {
		return nil
	}
	var active bool
	if err := tx.QueryRowContext(ctx, `SELECT NOT is_archived FROM epics WHERE id=$1`, epicID).Scan(&active); err != nil || !active {
		return ErrValidation
	}
	return nil
}

func replaceTags(ctx context.Context, tx *sql.Tx, cardID string, tagIDs []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM card_tags WHERE card_id=$1`, cardID); err != nil {
		return err
	}
	seen := make(map[string]bool)
	for _, tagID := range tagIDs {
		if tagID == "" || seen[tagID] {
			continue
		}
		seen[tagID] = true
		if _, err := tx.ExecContext(ctx, `INSERT INTO card_tags(card_id,tag_id) VALUES($1,$2)`, cardID, tagID); err != nil {
			return err
		}
	}
	return nil
}

func recordActivity(ctx context.Context, tx *sql.Tx, actor Actor, cardID, action string, changes map[string]any) error {
	if actor.UserID == "" {
		return ErrValidation
	}
	payload, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	var token any
	if actor.TokenID != "" {
		token = actor.TokenID
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO card_activity(card_id,actor_user_id,actor_token_id,action,changes) VALUES($1,$2,$3,$4,$5)`, cardID, actor.UserID, token, action, payload)
	return err
}

func movePosition(ctx context.Context, tx *sql.Tx, columnID, excludeID, beforeID, afterID string) (string, error) {
	if beforeID == "" && afterID == "" {
		var result string
		err := tx.QueryRowContext(ctx, `SELECT (COALESCE(max(position),0)+1000)::text FROM cards WHERE column_id=$1 AND id<>$2 AND NOT is_archived`, columnID, excludeID).Scan(&result)
		return result, err
	}
	referenceID, direction := beforeID, "before"
	if afterID != "" {
		referenceID, direction = afterID, "after"
	}
	var reference float64
	if err := tx.QueryRowContext(ctx, `SELECT position::float8 FROM cards WHERE id=$1 AND column_id=$2 AND NOT is_archived`, referenceID, columnID).Scan(&reference); err != nil {
		return "", ErrValidation
	}
	var neighbor sql.NullFloat64
	if direction == "before" {
		if err := tx.QueryRowContext(ctx, `SELECT max(position)::float8 FROM cards WHERE column_id=$1 AND id<>$2 AND position<$3`, columnID, excludeID, reference).Scan(&neighbor); err != nil {
			return "", err
		}
		if neighbor.Valid {
			reference = (reference + neighbor.Float64) / 2
		} else {
			reference -= 1000
		}
	} else {
		if err := tx.QueryRowContext(ctx, `SELECT min(position)::float8 FROM cards WHERE column_id=$1 AND id<>$2 AND position>$3`, columnID, excludeID, reference).Scan(&neighbor); err != nil {
			return "", err
		}
		if neighbor.Valid {
			reference = (reference + neighbor.Float64) / 2
		} else {
			reference += 1000
		}
	}
	return strconv.FormatFloat(reference, 'f', 15, 64), nil
}

func value(pointer *string) string {
	if pointer == nil {
		return ""
	}
	return *pointer
}

type listCursor struct {
	ColumnPosition int    `json:"c"`
	Position       string `json:"p"`
	ID             string `json:"i"`
}

func encodeCursor(cursor listCursor) string {
	raw, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(value string) (listCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return listCursor{}, err
	}
	var cursor listCursor
	err = json.Unmarshal(raw, &cursor)
	if err != nil || cursor.ID == "" || cursor.Position == "" {
		return listCursor{}, ErrValidation
	}
	return cursor, nil
}
