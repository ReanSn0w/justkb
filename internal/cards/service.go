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

	"github.com/ReanSn0w/justkb/internal/archive"
	appdb "github.com/ReanSn0w/justkb/internal/database"
	"github.com/ReanSn0w/justkb/internal/ids"
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

type Tag struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type CardEpic struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type CardGroup struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type CardAssignee struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
}

type Card struct {
	ID          string         `json:"id"`
	Number      int64          `json:"number"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	ColumnID    string         `json:"column_id"`
	EpicID      *string        `json:"epic_id"`
	GroupID     *string        `json:"group_id"`
	AssigneeID  *string        `json:"assignee_id"`
	AssigneeIDs []string       `json:"assignee_ids"`
	Position    string         `json:"position"`
	DueAt       *time.Time     `json:"due_at"`
	Version     int64          `json:"version"`
	IsArchived  bool           `json:"is_archived"`
	CreatedBy   string         `json:"created_by"`
	TagIDs      []string       `json:"tag_ids"`
	Tags        []Tag          `json:"tags"`
	Epic        *CardEpic      `json:"epic"`
	Group       *CardGroup     `json:"group"`
	Assignee    *CardAssignee  `json:"assignee"`
	Assignees   []CardAssignee `json:"assignees"`
	Attachments []Attachment   `json:"attachments"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type Activity struct {
	ID           string         `json:"id"`
	CardID       string         `json:"card_id"`
	ActorUserID  *string        `json:"actor_user_id"`
	ActorTokenID *string        `json:"actor_token_id"`
	Action       string         `json:"action"`
	Changes      map[string]any `json:"changes"`
	CreatedAt    time.Time      `json:"created_at"`
}

type MovementColumn struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type MovementActor struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type MovementToken struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Movement struct {
	ID               string         `json:"id"`
	CardID           string         `json:"card_id"`
	CardNumber       int64          `json:"card_number"`
	CardTitle        string         `json:"card_title"`
	FromColumn       MovementColumn `json:"from_column"`
	ToColumn         MovementColumn `json:"to_column"`
	Actor            MovementActor  `json:"actor"`
	ActorToken       *MovementToken `json:"actor_token,omitempty"`
	CurrentAssignee  *CardAssignee  `json:"current_assignee,omitempty"`
	CurrentAssignees []CardAssignee `json:"current_assignees"`
	Project          *CardEpic      `json:"project,omitempty"`
	Group            *CardGroup     `json:"group,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
}

type MovementFilters struct {
	From        time.Time
	To          time.Time
	ActorUserID string
	AssigneeID  string
	EpicID      string
	GroupID     string
	CardID      string
	Limit       int
	Cursor      string
}

type MovementPage struct {
	PeriodFrom time.Time  `json:"period_from"`
	PeriodTo   time.Time  `json:"period_to"`
	Items      []Movement `json:"items"`
	NextCursor string     `json:"next_cursor,omitempty"`
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
	AssigneeIDs []string
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
	AssigneeIDs     []string
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
	if actor.UserID == "" || strings.TrimSpace(input.Title) == "" || input.ColumnID == "" || (input.AssigneeID != "" && input.AssigneeIDs != nil) {
		return Card{}, ErrValidation
	}
	tx, err := appdb.BeginWrite(ctx, s.db)
	if err != nil {
		return Card{}, err
	}
	defer tx.Rollback()
	if err := validateEpic(ctx, tx, input.EpicID); err != nil {
		return Card{}, err
	}
	var columnExists bool
	if err := tx.QueryRowContext(ctx, `SELECT true FROM board_columns WHERE id=$1`, input.ColumnID).Scan(&columnExists); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Card{}, ErrValidation
		}
		return Card{}, err
	}
	var position string
	if err := tx.QueryRowContext(ctx, `SELECT CAST(COALESCE(min(position) - 1000, 1000) AS TEXT) FROM cards WHERE column_id=$1 AND NOT is_archived`, input.ColumnID).Scan(&position); err != nil {
		return Card{}, err
	}
	var number int64
	if err := tx.QueryRowContext(ctx, `UPDATE card_number_sequence SET next_number=next_number+1 WHERE singleton=1 RETURNING next_number-1`).Scan(&number); err != nil {
		return Card{}, err
	}
	assigneeID := input.AssigneeID
	if input.AssigneeIDs != nil {
		if err := validateAssignees(ctx, tx, input.AssigneeIDs); err != nil {
			return Card{}, err
		}
		if len(input.AssigneeIDs) > 0 {
			assigneeID = input.AssigneeIDs[0]
		}
	}
	var card Card
	err = tx.QueryRowContext(ctx, `
		INSERT INTO cards (id,number,title,description,column_id,epic_id,group_id,assignee_id,position,due_at,created_by)
		VALUES ($1,$2,$3,$4,$5,NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),$9,$10,$11)
		RETURNING id,number,title,description,column_id,epic_id,group_id,assignee_id,CAST(position AS TEXT),due_at,version,is_archived,created_by,created_at,updated_at`,
		ids.New(), number, strings.TrimSpace(input.Title), input.Description, input.ColumnID, input.EpicID, input.GroupID, assigneeID, position, input.DueAt, actor.UserID,
	).Scan(&card.ID, &card.Number, &card.Title, &card.Description, &card.ColumnID, &card.EpicID, &card.GroupID, &card.AssigneeID, &card.Position, &card.DueAt, &card.Version, &card.IsArchived, &card.CreatedBy, &card.CreatedAt, &card.UpdatedAt)
	if err != nil {
		return Card{}, err
	}
	if len(input.AssigneeIDs) > 1 {
		if err := replaceAdditionalAssignees(ctx, tx, card.ID, input.AssigneeIDs[1:]); err != nil {
			return Card{}, err
		}
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
	query := cardSelect + ` WHERE c.id=$1`
	argument := idOrNumber
	if strings.HasPrefix(strings.ToUpper(idOrNumber), "JKB-") {
		query = cardSelect + ` WHERE c.number=$1`
		argument = strings.TrimPrefix(strings.ToUpper(idOrNumber), "JKB-")
	}
	row := s.db.QueryRowContext(ctx, query, argument)
	return scanCard(row)
}

const assigneeFilter = ` AND (c.assignee_id=$%[1]d OR EXISTS (SELECT 1 FROM card_additional_assignees ca WHERE ca.card_id=c.id AND ca.user_id=$%[1]d))`

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
		query += ` AND c.assignee_id IS NULL AND NOT EXISTS (SELECT 1 FROM card_additional_assignees ca WHERE ca.card_id=c.id)`
	} else if filters.AssigneeID == "me" {
		if filters.CurrentUserID == "" {
			return Page{}, ErrValidation
		}
		add(assigneeFilter, filters.CurrentUserID)
	} else if filters.AssigneeID != "" {
		add(assigneeFilter, filters.AssigneeID)
	}
	if filters.ColumnID != "" {
		add(` AND c.column_id=$%d`, filters.ColumnID)
	}
	if strings.TrimSpace(filters.Query) != "" {
		args = append(args, strings.TrimSpace(filters.Query))
		index := len(args)
		query += fmt.Sprintf(` AND (CAST(c.number AS TEXT)=$%d OR instr(lower(c.title||' '||c.description),lower($%d))>0)`, index, index)
	}
	for _, tagID := range filters.TagIDs {
		add(` AND EXISTS(SELECT 1 FROM card_tags ct WHERE ct.card_id=c.id AND ct.tag_id=$%d)`, tagID)
	}
	if filters.Cursor != "" {
		cursor, err := decodeCursor(filters.Cursor)
		if err != nil {
			return Page{}, ErrValidation
		}
		args = append(args, cursor.ColumnPosition, cursor.Position, cursor.ID)
		query += fmt.Sprintf(` AND (ordering_column.position,c.position,c.id) > ($%d,CAST($%d AS REAL),$%d)`, len(args)-2, len(args)-1, len(args))
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
	if input.ExpectedVersion <= 0 || (input.AssigneeID != nil && input.AssigneeIDs != nil) {
		return Card{}, ErrValidation
	}
	tx, err := appdb.BeginWrite(ctx, s.db)
	if err != nil {
		return Card{}, err
	}
	defer tx.Rollback()
	current, err := getForUpdate(ctx, tx, cardID)
	if err != nil {
		return Card{}, err
	}
	if current.IsArchived {
		return Card{}, archive.ErrReadOnly
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
	assigneeIDs := input.AssigneeIDs
	if input.AssigneeID != nil && *input.AssigneeID != assigneeID {
		// A legacy write replaces only the primary assignee, preserving the others.
		assigneeIDs = make([]string, 0, len(current.AssigneeIDs)+1)
		if *input.AssigneeID != "" {
			assigneeIDs = append(assigneeIDs, *input.AssigneeID)
		}
		for _, id := range current.AssigneeIDs {
			if id != assigneeID && id != *input.AssigneeID {
				assigneeIDs = append(assigneeIDs, id)
			}
		}
	}
	if assigneeIDs != nil {
		if err := validateAssignees(ctx, tx, assigneeIDs); err != nil {
			return Card{}, err
		}
		assigneeID = ""
		if len(assigneeIDs) > 0 {
			assigneeID, assigneeIDs = assigneeIDs[0], assigneeIDs[1:]
		}
	}
	if input.DueAt != nil {
		dueAt = *input.DueAt
	}
	result, err := tx.ExecContext(ctx, `
		UPDATE cards SET title=$3,description=$4,epic_id=NULLIF($5,''),group_id=NULLIF($6,''),
		assignee_id=NULLIF($7,''),due_at=$8,version=version+1,updated_at=CURRENT_TIMESTAMP
		WHERE id=$1 AND version=$2`, cardID, input.ExpectedVersion, title, description, epicID, groupID, assigneeID, dueAt)
	if err != nil {
		return Card{}, err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return Card{}, ErrVersionConflict
	}
	if assigneeIDs != nil {
		if err := replaceAdditionalAssignees(ctx, tx, cardID, assigneeIDs); err != nil {
			return Card{}, err
		}
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
	tx, err := appdb.BeginWrite(ctx, s.db)
	if err != nil {
		return Card{}, err
	}
	defer tx.Rollback()
	current, err := getForUpdate(ctx, tx, cardID)
	if err != nil {
		return Card{}, err
	}
	if current.IsArchived {
		return Card{}, archive.ErrReadOnly
	}
	if current.Version != input.ExpectedVersion {
		return Card{}, ErrVersionConflict
	}
	var exists bool
	if err := tx.QueryRowContext(ctx, `SELECT true FROM board_columns WHERE id=$1`, input.TargetColumnID).Scan(&exists); err != nil {
		return Card{}, ErrValidation
	}
	position, err := movePosition(ctx, tx, input.TargetColumnID, cardID, input.BeforeCardID, input.AfterCardID)
	if err != nil {
		return Card{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE cards SET column_id=$3,position=$4,version=version+1,updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND version=$2`, cardID, input.ExpectedVersion, input.TargetColumnID, position)
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
	tx, err := appdb.BeginWrite(ctx, s.db)
	if err != nil {
		return Card{}, err
	}
	defer tx.Rollback()
	if err := archive.Card(ctx, tx, archive.Actor{UserID: actor.UserID, TokenID: actor.TokenID}, cardID, expectedVersion, archived, "", time.Now()); err != nil {
		if errors.Is(err, archive.ErrVersionConflict) {
			return Card{}, ErrVersionConflict
		}
		if errors.Is(err, archive.ErrNotFound) {
			return Card{}, ErrNotFound
		}
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

func (s *Service) ListMovements(ctx context.Context, filters MovementFilters) (MovementPage, error) {
	if filters.From.IsZero() || filters.To.IsZero() || !filters.From.Before(filters.To) {
		return MovementPage{}, ErrValidation
	}
	if filters.Limit <= 0 {
		filters.Limit = 100
	}
	if filters.Limit > 500 {
		return MovementPage{}, ErrValidation
	}
	query := `
		SELECT activity.id,activity.card_id,card.number,card.title,
			from_column.id,from_column.name,to_column.id,to_column.name,
			actor.id,actor.display_name,activity.actor_token_id,actor_token.name,
			assignee.id,assignee.display_name,assignee.avatar_url,
			project.id,project.name,project.color,
			card_group.id,card_group.name,activity.created_at,
			(SELECT json_group_array(json_object('id',u.id,'display_name',u.display_name,'avatar_url',u.avatar_url) ORDER BY ca.position)
			 FROM card_additional_assignees ca JOIN users u ON u.id=ca.user_id
			 WHERE ca.card_id=card.id AND ca.user_id IS NOT card.assignee_id)
		FROM card_activity activity
		JOIN cards card ON card.id=activity.card_id
		JOIN users actor ON actor.id=activity.actor_user_id
		LEFT JOIN api_tokens actor_token ON actor_token.id=activity.actor_token_id
		LEFT JOIN users assignee ON assignee.id=card.assignee_id
		LEFT JOIN epics project ON project.id=card.epic_id
		LEFT JOIN user_groups card_group ON card_group.id=card.group_id
		LEFT JOIN board_columns from_column ON from_column.id=json_extract(activity.changes,'$.from_column_id')
		LEFT JOIN board_columns to_column ON to_column.id=json_extract(activity.changes,'$.to_column_id')
		WHERE activity.action='moved' AND datetime(activity.created_at) >= datetime($1) AND datetime(activity.created_at) < datetime($2)`
	args := []any{sqliteTime(filters.From), sqliteTime(filters.To)}
	add := func(clause string, value any) {
		args = append(args, value)
		query += fmt.Sprintf(clause, len(args))
	}
	if filters.ActorUserID != "" {
		add(` AND activity.actor_user_id=$%d`, filters.ActorUserID)
	}
	if filters.AssigneeID != "" {
		add(` AND (card.assignee_id=$%[1]d OR EXISTS (SELECT 1 FROM card_additional_assignees ca WHERE ca.card_id=card.id AND ca.user_id=$%[1]d))`, filters.AssigneeID)
	}
	if filters.EpicID != "" {
		add(` AND card.epic_id=$%d`, filters.EpicID)
	}
	if filters.GroupID != "" {
		add(` AND card.group_id=$%d`, filters.GroupID)
	}
	if filters.CardID != "" {
		add(` AND card.id=$%d`, filters.CardID)
	}
	if filters.Cursor != "" {
		cursor, err := decodeMovementCursor(filters.Cursor)
		if err != nil {
			return MovementPage{}, ErrValidation
		}
		args = append(args, sqliteTime(cursor.CreatedAt), cursor.ID)
		query += fmt.Sprintf(` AND (datetime(activity.created_at),activity.id) < (datetime($%d),$%d)`, len(args)-1, len(args))
	}
	args = append(args, filters.Limit+1)
	query += fmt.Sprintf(` ORDER BY datetime(activity.created_at) DESC,activity.id DESC LIMIT $%d`, len(args))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return MovementPage{}, err
	}
	defer rows.Close()
	items := make([]Movement, 0, filters.Limit+1)
	for rows.Next() {
		var item Movement
		var fromID, fromName, toID, toName sql.NullString
		var tokenID, tokenName sql.NullString
		var assigneeID, assigneeName, assigneeAvatar sql.NullString
		var additional []byte
		var projectID, projectName, projectColor sql.NullString
		var groupID, groupName sql.NullString
		if err := rows.Scan(
			&item.ID, &item.CardID, &item.CardNumber, &item.CardTitle,
			&fromID, &fromName, &toID, &toName,
			&item.Actor.ID, &item.Actor.DisplayName, &tokenID, &tokenName,
			&assigneeID, &assigneeName, &assigneeAvatar,
			&projectID, &projectName, &projectColor,
			&groupID, &groupName, &item.CreatedAt, &additional,
		); err != nil {
			return MovementPage{}, err
		}
		item.FromColumn = MovementColumn{ID: fromID.String, Name: fromName.String}
		item.ToColumn = MovementColumn{ID: toID.String, Name: toName.String}
		if tokenID.Valid {
			item.ActorToken = &MovementToken{ID: tokenID.String, Name: tokenName.String}
		}
		if assigneeID.Valid {
			item.CurrentAssignee = &CardAssignee{ID: assigneeID.String, DisplayName: assigneeName.String}
			if assigneeAvatar.Valid {
				item.CurrentAssignee.AvatarURL = &assigneeAvatar.String
			}
		}
		var others []CardAssignee
		if err := json.Unmarshal(additional, &others); err != nil {
			return MovementPage{}, err
		}
		item.CurrentAssignees = make([]CardAssignee, 0, len(others)+1)
		if item.CurrentAssignee != nil {
			item.CurrentAssignees = append(item.CurrentAssignees, *item.CurrentAssignee)
		}
		item.CurrentAssignees = append(item.CurrentAssignees, others...)
		if projectID.Valid {
			item.Project = &CardEpic{ID: projectID.String, Name: projectName.String, Color: projectColor.String}
		}
		if groupID.Valid {
			item.Group = &CardGroup{ID: groupID.String, Name: groupName.String}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return MovementPage{}, err
	}
	page := MovementPage{PeriodFrom: filters.From, PeriodTo: filters.To, Items: items}
	if len(items) > filters.Limit {
		last := items[filters.Limit-1]
		page.Items = items[:filters.Limit]
		page.NextCursor = encodeMovementCursor(movementCursor{CreatedAt: last.CreatedAt, ID: last.ID})
	}
	return page, nil
}

const cardSelect = `
	SELECT c.id,c.number,c.title,c.description,c.column_id,c.epic_id,c.group_id,c.assignee_id,CAST(c.position AS TEXT),c.due_at,c.version,c.is_archived,c.created_by,c.created_at,c.updated_at,
	COALESCE((SELECT json_group_array(json_object('id',t.id,'name',t.name,'color',t.color)) FROM card_tags ct JOIN tags t ON t.id=ct.tag_id WHERE ct.card_id=c.id),'[]'),
	COALESCE((SELECT json_group_array(json_object('id',a.id,'original_name',a.original_name,'media_type',a.media_type,'size',a.size,'created_by',a.created_by,'created_at',strftime('%Y-%m-%dT%H:%M:%fZ',a.created_at))) FROM attachments a WHERE a.card_id=c.id),'[]'),
	(SELECT json_object('id',e.id,'name',e.name,'color',e.color) FROM epics e WHERE e.id=c.epic_id),
	(SELECT json_object('id',g.id,'name',g.name,'color',g.color) FROM user_groups g WHERE g.id=c.group_id),
	(SELECT json_object('id',u.id,'display_name',u.display_name,'avatar_url',u.avatar_url) FROM users u WHERE u.id=c.assignee_id),
	(SELECT json_group_array(json_object('id',u.id,'display_name',u.display_name,'avatar_url',u.avatar_url) ORDER BY ca.position)
	 FROM card_additional_assignees ca JOIN users u ON u.id=ca.user_id
	 WHERE ca.card_id=c.id AND ca.user_id IS NOT c.assignee_id)
	FROM cards c`

type scanner interface{ Scan(...any) error }

func scanCard(row scanner) (Card, error) {
	var card Card
	var tags, attachments, epic, group, assignee, additional []byte
	err := row.Scan(&card.ID, &card.Number, &card.Title, &card.Description, &card.ColumnID, &card.EpicID, &card.GroupID, &card.AssigneeID, &card.Position, &card.DueAt, &card.Version, &card.IsArchived, &card.CreatedBy, &card.CreatedAt, &card.UpdatedAt, &tags, &attachments, &epic, &group, &assignee, &additional)
	if errors.Is(err, sql.ErrNoRows) {
		return Card{}, ErrNotFound
	}
	if err != nil {
		return Card{}, err
	}
	if err := json.Unmarshal(tags, &card.Tags); err != nil {
		return Card{}, err
	}
	card.TagIDs = make([]string, 0, len(card.Tags))
	for _, tag := range card.Tags {
		card.TagIDs = append(card.TagIDs, tag.ID)
	}
	if err := json.Unmarshal(attachments, &card.Attachments); err != nil {
		return Card{}, err
	}
	if len(epic) > 0 && string(epic) != "null" {
		if err := json.Unmarshal(epic, &card.Epic); err != nil {
			return Card{}, err
		}
	}
	if len(group) > 0 && string(group) != "null" {
		if err := json.Unmarshal(group, &card.Group); err != nil {
			return Card{}, err
		}
	}
	if len(assignee) > 0 && string(assignee) != "null" {
		if err := json.Unmarshal(assignee, &card.Assignee); err != nil {
			return Card{}, err
		}
	}
	var others []CardAssignee
	if err := json.Unmarshal(additional, &others); err != nil {
		return Card{}, err
	}
	card.Assignees = make([]CardAssignee, 0, len(others)+1)
	if card.Assignee != nil {
		card.Assignees = append(card.Assignees, *card.Assignee)
	}
	card.Assignees = append(card.Assignees, others...)
	card.AssigneeIDs = make([]string, 0, len(card.Assignees))
	for _, assignee := range card.Assignees {
		card.AssigneeIDs = append(card.AssigneeIDs, assignee.ID)
	}
	return card, nil
}

func validateAssignees(ctx context.Context, tx *sql.Tx, assigneeIDs []string) error {
	seen := make(map[string]bool, len(assigneeIDs))
	for _, id := range assigneeIDs {
		if id == "" || seen[id] {
			return ErrValidation
		}
		seen[id] = true
		var exists bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id=$1)`, id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrValidation
		}
	}
	return nil
}

func replaceAdditionalAssignees(ctx context.Context, tx *sql.Tx, cardID string, assigneeIDs []string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM card_additional_assignees WHERE card_id=$1`, cardID); err != nil {
		return err
	}
	for index, id := range assigneeIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO card_additional_assignees(card_id,user_id,position) VALUES($1,$2,$3)`, cardID, id, index+1); err != nil {
			return err
		}
	}
	return nil
}

func getForUpdate(ctx context.Context, tx *sql.Tx, cardID string) (Card, error) {
	row := tx.QueryRowContext(ctx, cardSelect+` WHERE c.id=$1`, cardID)
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
	_, err = tx.ExecContext(ctx, `INSERT INTO card_activity(id,card_id,actor_user_id,actor_token_id,action,changes) VALUES($1,$2,$3,$4,$5,$6)`, ids.New(), cardID, actor.UserID, token, action, payload)
	return err
}

func movePosition(ctx context.Context, tx *sql.Tx, columnID, excludeID, beforeID, afterID string) (string, error) {
	if beforeID == "" && afterID == "" {
		var result string
		err := tx.QueryRowContext(ctx, `SELECT CAST(COALESCE(max(position),0)+1000 AS TEXT) FROM cards WHERE column_id=$1 AND id<>$2 AND NOT is_archived`, columnID, excludeID).Scan(&result)
		return result, err
	}
	referenceID, direction := beforeID, "before"
	if afterID != "" {
		referenceID, direction = afterID, "after"
	}
	var reference float64
	if err := tx.QueryRowContext(ctx, `SELECT CAST(position AS REAL) FROM cards WHERE id=$1 AND column_id=$2 AND NOT is_archived`, referenceID, columnID).Scan(&reference); err != nil {
		return "", ErrValidation
	}
	var neighbor sql.NullFloat64
	if direction == "before" {
		if err := tx.QueryRowContext(ctx, `SELECT max(CAST(position AS REAL)) FROM cards WHERE column_id=$1 AND id<>$2 AND position<$3`, columnID, excludeID, reference).Scan(&neighbor); err != nil {
			return "", err
		}
		if neighbor.Valid {
			reference = (reference + neighbor.Float64) / 2
		} else {
			reference -= 1000
		}
	} else {
		if err := tx.QueryRowContext(ctx, `SELECT min(CAST(position AS REAL)) FROM cards WHERE column_id=$1 AND id<>$2 AND position>$3`, columnID, excludeID, reference).Scan(&neighbor); err != nil {
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

func sqliteTime(value time.Time) string {
	return value.UTC().Format("2006-01-02 15:04:05.999999999")
}

type listCursor struct {
	ColumnPosition int    `json:"c"`
	Position       string `json:"p"`
	ID             string `json:"i"`
}

type movementCursor struct {
	CreatedAt time.Time `json:"t"`
	ID        string    `json:"i"`
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

func encodeMovementCursor(cursor movementCursor) string {
	raw, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeMovementCursor(value string) (movementCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return movementCursor{}, err
	}
	var cursor movementCursor
	err = json.Unmarshal(raw, &cursor)
	if err != nil || cursor.ID == "" || cursor.CreatedAt.IsZero() {
		return movementCursor{}, ErrValidation
	}
	return cursor, nil
}
