package board

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ReanSn0w/justkb/internal/auth"
)

var (
	ErrNotFound       = errors.New("not found")
	ErrValidation     = errors.New("validation failed")
	ErrColumnNotEmpty = errors.New("column is not empty")
)

var colorPattern = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type User struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	DisplayName string   `json:"display_name"`
	Email       *string  `json:"email"`
	AvatarURL   *string  `json:"avatar_url"`
	Role        string   `json:"role"`
	IsActive    bool     `json:"is_active"`
	GroupIDs    []string `json:"group_ids"`
}

type Group struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Color       string  `json:"color"`
	Description *string `json:"description"`
}

type Column struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
	WIPLimit *int   `json:"wip_limit"`
}

type Epic struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	Color       string    `json:"color"`
	IsArchived  bool      `json:"is_archived"`
	CardCount   int       `json:"card_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Tag struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

type Service struct{ db *sql.DB }

func NewService(db *sql.DB) *Service { return &Service{db: db} }

func (s *Service) ListUsers(ctx context.Context, includeInactive bool, groupID string) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT u.id, u.username, u.display_name, u.email, u.avatar_url, u.role::text, u.is_active
		FROM users u
		WHERE ($1 OR u.is_active) AND ($2 = '' OR EXISTS (
			SELECT 1 FROM group_members gm WHERE gm.user_id = u.id AND gm.group_id = NULLIF($2, '')::uuid
		))
		ORDER BY lower(u.display_name), u.id`, includeInactive, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	users := make([]User, 0)
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Username, &user.DisplayName, &user.Email, &user.AvatarURL, &user.Role, &user.IsActive); err != nil {
			return nil, err
		}
		user.GroupIDs, err = s.userGroupIDs(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (s *Service) CreateUser(ctx context.Context, username, displayName, email, role, password string) (User, error) {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(displayName) == "" || (role != "admin" && role != "member") {
		return User{}, ErrValidation
	}
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return User{}, err
	}
	var emailValue any
	if strings.TrimSpace(email) != "" {
		emailValue = strings.TrimSpace(email)
	}
	var user User
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO users (username, display_name, email, role, password_hash)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, username, display_name, email, avatar_url, role::text, is_active`,
		strings.TrimSpace(username), strings.TrimSpace(displayName), emailValue, role, passwordHash,
	).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Email, &user.AvatarURL, &user.Role, &user.IsActive)
	user.GroupIDs = []string{}
	return user, err
}

func (s *Service) UpdateUser(ctx context.Context, id, displayName, email, role string, active bool) (User, error) {
	if strings.TrimSpace(displayName) == "" || (role != "admin" && role != "member") {
		return User{}, ErrValidation
	}
	var emailValue any
	if strings.TrimSpace(email) != "" {
		emailValue = strings.TrimSpace(email)
	}
	var user User
	err := s.db.QueryRowContext(ctx, `
		UPDATE users SET display_name = $2, email = $3, role = $4, is_active = $5, updated_at = now()
		WHERE id = $1
		RETURNING id, username, display_name, email, avatar_url, role::text, is_active`,
		id, strings.TrimSpace(displayName), emailValue, role, active,
	).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Email, &user.AvatarURL, &user.Role, &user.IsActive)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err == nil {
		user.GroupIDs, err = s.userGroupIDs(ctx, user.ID)
	}
	return user, err
}

func (s *Service) SetUserGroups(ctx context.Context, userID string, groupIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE user_id = $1`, userID); err != nil {
		return err
	}
	seen := make(map[string]bool)
	for _, id := range groupIDs {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if _, err := tx.ExecContext(ctx, `INSERT INTO group_members (group_id, user_id) VALUES ($1, $2)`, id, userID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Service) ResetUserPassword(ctx context.Context, userID, password string) error {
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, userID, hash)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM web_sessions WHERE user_id = $1`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) ResetUserPasswordByUsername(ctx context.Context, username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return ErrValidation
	}
	var userID string
	if err := s.db.QueryRowContext(ctx, `SELECT id FROM users WHERE lower(username)=lower($1)`, username).Scan(&userID); errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	} else if err != nil {
		return err
	}
	return s.ResetUserPassword(ctx, userID, password)
}

func (s *Service) ListGroups(ctx context.Context) ([]Group, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, color, description FROM user_groups ORDER BY lower(name), id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Group, 0)
	for rows.Next() {
		var item Group
		if err := rows.Scan(&item.ID, &item.Name, &item.Color, &item.Description); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) SaveGroup(ctx context.Context, id, name, color string, description *string) (Group, error) {
	if strings.TrimSpace(name) == "" || !colorPattern.MatchString(color) {
		return Group{}, ErrValidation
	}
	var group Group
	var err error
	if id == "" {
		err = s.db.QueryRowContext(ctx, `INSERT INTO user_groups (name, color, description) VALUES ($1, $2, $3) RETURNING id, name, color, description`, strings.TrimSpace(name), color, description).Scan(&group.ID, &group.Name, &group.Color, &group.Description)
	} else {
		err = s.db.QueryRowContext(ctx, `UPDATE user_groups SET name=$2, color=$3, description=$4, updated_at=now() WHERE id=$1 RETURNING id, name, color, description`, id, strings.TrimSpace(name), color, description).Scan(&group.ID, &group.Name, &group.Color, &group.Description)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return Group{}, ErrNotFound
	}
	return group, err
}

func (s *Service) ListColumns(ctx context.Context) ([]Column, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, position, wip_limit FROM board_columns ORDER BY position, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Column, 0)
	for rows.Next() {
		var item Column
		if err := rows.Scan(&item.ID, &item.Name, &item.Position, &item.WIPLimit); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) SaveColumn(ctx context.Context, id, name string, wipLimit *int) (Column, error) {
	if strings.TrimSpace(name) == "" || (wipLimit != nil && *wipLimit <= 0) {
		return Column{}, ErrValidation
	}
	var column Column
	var err error
	if id == "" {
		err = s.db.QueryRowContext(ctx, `INSERT INTO board_columns (name, position, wip_limit) VALUES ($1, COALESCE((SELECT max(position) + 100 FROM board_columns), 100), $2) RETURNING id, name, position, wip_limit`, strings.TrimSpace(name), wipLimit).Scan(&column.ID, &column.Name, &column.Position, &column.WIPLimit)
	} else {
		err = s.db.QueryRowContext(ctx, `UPDATE board_columns SET name=$2, wip_limit=$3, updated_at=now() WHERE id=$1 RETURNING id, name, position, wip_limit`, id, strings.TrimSpace(name), wipLimit).Scan(&column.ID, &column.Name, &column.Position, &column.WIPLimit)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return Column{}, ErrNotFound
	}
	return column, err
}

func (s *Service) ReorderColumns(ctx context.Context, ids []string) error {
	current, err := s.ListColumns(ctx)
	if err != nil {
		return err
	}
	if len(ids) != len(current) {
		return ErrValidation
	}
	wanted := append([]string(nil), ids...)
	sort.Strings(wanted)
	existing := make([]string, 0, len(current))
	for _, column := range current {
		existing = append(existing, column.ID)
	}
	sort.Strings(existing)
	if strings.Join(wanted, ",") != strings.Join(existing, ",") {
		return ErrValidation
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE board_columns SET position = position + 1000000, updated_at = now()`); err != nil {
		return err
	}
	for index, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE board_columns SET position = $2, updated_at = now() WHERE id = $1`, id, (index+1)*100); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Service) DeleteColumn(ctx context.Context, id string) error {
	var count int
	if err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM cards WHERE column_id = $1`, id).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return ErrColumnNotEmpty
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM board_columns WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if affected, _ := result.RowsAffected(); affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ListEpics(ctx context.Context, includeArchived bool) ([]Epic, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id,e.name,e.description,e.color,e.is_archived,e.created_at,e.updated_at,
		       (SELECT count(*) FROM cards c WHERE c.epic_id=e.id AND NOT c.is_archived)
		FROM epics e WHERE $1 OR NOT e.is_archived
		ORDER BY e.is_archived,lower(e.name),e.id`, includeArchived)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Epic, 0)
	for rows.Next() {
		var item Epic
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Color, &item.IsArchived, &item.CreatedAt, &item.UpdatedAt, &item.CardCount); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) GetEpic(ctx context.Context, id string) (Epic, error) {
	var item Epic
	err := s.db.QueryRowContext(ctx, `
		SELECT e.id,e.name,e.description,e.color,e.is_archived,e.created_at,e.updated_at,
		       (SELECT count(*) FROM cards c WHERE c.epic_id=e.id AND NOT c.is_archived)
		FROM epics e WHERE e.id=$1`, id).Scan(
		&item.ID, &item.Name, &item.Description, &item.Color, &item.IsArchived,
		&item.CreatedAt, &item.UpdatedAt, &item.CardCount,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Epic{}, ErrNotFound
	}
	return item, err
}

func (s *Service) SaveEpic(ctx context.Context, id, name, color string, description *string, archived bool) (Epic, error) {
	if strings.TrimSpace(name) == "" || !colorPattern.MatchString(color) {
		return Epic{}, ErrValidation
	}
	var epic Epic
	var err error
	if id == "" {
		err = s.db.QueryRowContext(ctx, `INSERT INTO epics (name, description, color, is_archived) VALUES ($1,$2,$3,$4) RETURNING id,name,description,color,is_archived,created_at,updated_at`, strings.TrimSpace(name), description, color, archived).Scan(&epic.ID, &epic.Name, &epic.Description, &epic.Color, &epic.IsArchived, &epic.CreatedAt, &epic.UpdatedAt)
	} else {
		err = s.db.QueryRowContext(ctx, `UPDATE epics SET name=$2,description=$3,color=$4,is_archived=$5,updated_at=now() WHERE id=$1 RETURNING id,name,description,color,is_archived,created_at,updated_at`, id, strings.TrimSpace(name), description, color, archived).Scan(&epic.ID, &epic.Name, &epic.Description, &epic.Color, &epic.IsArchived, &epic.CreatedAt, &epic.UpdatedAt)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return Epic{}, ErrNotFound
	}
	return epic, err
}

func (s *Service) ListTags(ctx context.Context) ([]Tag, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, color FROM tags ORDER BY lower(name), id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Tag, 0)
	for rows.Next() {
		var item Tag
		if err := rows.Scan(&item.ID, &item.Name, &item.Color); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) SaveTag(ctx context.Context, id, name, color string) (Tag, error) {
	if strings.TrimSpace(name) == "" || !colorPattern.MatchString(color) {
		return Tag{}, ErrValidation
	}
	var tag Tag
	var err error
	if id == "" {
		err = s.db.QueryRowContext(ctx, `INSERT INTO tags (name,color) VALUES ($1,$2) RETURNING id,name,color`, strings.TrimSpace(name), color).Scan(&tag.ID, &tag.Name, &tag.Color)
	} else {
		err = s.db.QueryRowContext(ctx, `UPDATE tags SET name=$2,color=$3,updated_at=now() WHERE id=$1 RETURNING id,name,color`, id, strings.TrimSpace(name), color).Scan(&tag.ID, &tag.Name, &tag.Color)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return Tag{}, ErrNotFound
	}
	return tag, err
}

func (s *Service) userGroupIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT group_id FROM group_members WHERE user_id=$1 ORDER BY group_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func WrapValidation(field string) error { return fmt.Errorf("%w: %s", ErrValidation, field) }
