package comments

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/ReanSn0w/justkb/internal/ids"
)

var (
	ErrNotFound   = errors.New("comment owner not found")
	ErrValidation = errors.New("comment validation failed")
)

type Actor struct {
	UserID  string
	TokenID string
}

type Author struct {
	ID          string  `json:"id"`
	DisplayName string  `json:"display_name"`
	AvatarURL   *string `json:"avatar_url"`
}

type Comment struct {
	ID            string    `json:"id"`
	CardID        *string   `json:"card_id,omitempty"`
	EpicID        *string   `json:"epic_id,omitempty"`
	AuthorUserID  string    `json:"author_user_id"`
	AuthorTokenID *string   `json:"author_token_id,omitempty"`
	Body          string    `json:"body"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	Author        Author    `json:"author"`
}

type Service struct{ db *sql.DB }

func NewService(db *sql.DB) *Service { return &Service{db: db} }

func (s *Service) ListCard(ctx context.Context, cardID string) ([]Comment, error) {
	return s.list(ctx, cardID, "")
}

func (s *Service) ListEpic(ctx context.Context, epicID string) ([]Comment, error) {
	return s.list(ctx, "", epicID)
}

func (s *Service) list(ctx context.Context, cardID, epicID string) ([]Comment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id,c.card_id,c.epic_id,c.author_user_id,c.author_token_id,c.body,c.created_at,c.updated_at,
		       u.id,u.display_name,u.avatar_url
		FROM comments c JOIN users u ON u.id=c.author_user_id
		WHERE ($1 <> '' AND c.card_id=NULLIF($1,''))
		   OR ($2 <> '' AND c.epic_id=NULLIF($2,''))
		ORDER BY c.created_at,c.id`, cardID, epicID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Comment, 0)
	for rows.Next() {
		var item Comment
		if err := rows.Scan(&item.ID, &item.CardID, &item.EpicID, &item.AuthorUserID, &item.AuthorTokenID,
			&item.Body, &item.CreatedAt, &item.UpdatedAt, &item.Author.ID, &item.Author.DisplayName, &item.Author.AvatarURL); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) CreateCard(ctx context.Context, cardID string, actor Actor, body string) (Comment, error) {
	return s.create(ctx, cardID, "", actor, body)
}

func (s *Service) CreateEpic(ctx context.Context, epicID string, actor Actor, body string) (Comment, error) {
	return s.create(ctx, "", epicID, actor, body)
}

func (s *Service) create(ctx context.Context, cardID, epicID string, actor Actor, body string) (Comment, error) {
	body = strings.TrimSpace(body)
	if (cardID == "") == (epicID == "") || actor.UserID == "" || body == "" {
		return Comment{}, ErrValidation
	}
	var exists bool
	if cardID != "" {
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM cards WHERE id=$1)`, cardID).Scan(&exists); err != nil {
			return Comment{}, err
		}
	} else if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM epics WHERE id=$1)`, epicID).Scan(&exists); err != nil {
		return Comment{}, err
	}
	if !exists {
		return Comment{}, ErrNotFound
	}
	var tokenID any
	if actor.TokenID != "" {
		tokenID = actor.TokenID
	}
	commentID := ids.New()
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO comments(id,card_id,epic_id,author_user_id,author_token_id,body)
		VALUES($1,NULLIF($2,''),NULLIF($3,''),$4,$5,$6)`,
		commentID, cardID, epicID, actor.UserID, tokenID, body,
	); err != nil {
		return Comment{}, err
	}
	var item Comment
	err := s.db.QueryRowContext(ctx, `
		SELECT c.id,c.card_id,c.epic_id,c.author_user_id,c.author_token_id,c.body,c.created_at,c.updated_at,
		       u.id,u.display_name,u.avatar_url
		FROM comments c JOIN users u ON u.id=c.author_user_id
		WHERE c.id=$1`, commentID,
	).Scan(&item.ID, &item.CardID, &item.EpicID, &item.AuthorUserID, &item.AuthorTokenID,
		&item.Body, &item.CreatedAt, &item.UpdatedAt, &item.Author.ID, &item.Author.DisplayName, &item.Author.AvatarURL)
	if err != nil {
		return Comment{}, err
	}
	return item, nil
}
