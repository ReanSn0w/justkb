package tokens

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/ReanSn0w/justkb/internal/auth"
	appdb "github.com/ReanSn0w/justkb/internal/database"
	"github.com/ReanSn0w/justkb/internal/ids"
)

var ErrBotTokenConflict = errors.New("bot token changed; reload its metadata before rotating")

type CreatedBot struct {
	UserID string  `json:"user_id"`
	Token  Created `json:"token"`
}

func requireBotAdmin(ctx context.Context, tx *sql.Tx, actorID string) error {
	var allowed bool
	if err := tx.QueryRowContext(ctx, `SELECT account_type='human' AND role='admin' AND is_active FROM users WHERE id=?`, actorID).Scan(&allowed); errors.Is(err, sql.ErrNoRows) {
		return auth.ErrForbidden
	} else if err != nil {
		return err
	}
	if !allowed {
		return auth.ErrForbidden
	}
	return nil
}

func (s *Service) CreateBot(ctx context.Context, actorID, displayName string) (CreatedBot, error) {
	displayName = strings.TrimSpace(displayName)
	if displayName == "" || len([]rune(displayName)) > 200 {
		return CreatedBot{}, auth.ErrValidation
	}
	tx, err := appdb.BeginWrite(ctx, s.db)
	if err != nil {
		return CreatedBot{}, err
	}
	defer tx.Rollback()
	if err = requireBotAdmin(ctx, tx, actorID); err != nil {
		return CreatedBot{}, err
	}
	id := ids.New()
	if _, err = tx.ExecContext(ctx, `INSERT INTO users(id,username,display_name,role,password_hash,account_type) VALUES(?,?,?,'member','','bot')`, id, "bot-"+id, displayName); err != nil {
		return CreatedBot{}, err
	}
	created, err := createInTx(ctx, tx, id, "MCP", []string{"cards:read", "cards:write"}, nil)
	if err != nil {
		return CreatedBot{}, err
	}
	if err = tx.Commit(); err != nil {
		return CreatedBot{}, err
	}
	return CreatedBot{UserID: id, Token: created}, nil
}

func (s *Service) RotateBot(ctx context.Context, actorID, userID, expectedTokenID string) (Created, error) {
	if expectedTokenID == "" {
		return Created{}, auth.ErrValidation
	}
	tx, err := appdb.BeginWrite(ctx, s.db)
	if err != nil {
		return Created{}, err
	}
	defer tx.Rollback()
	if err = requireBotAdmin(ctx, tx, actorID); err != nil {
		return Created{}, err
	}
	var current string
	err = tx.QueryRowContext(ctx, `SELECT t.id FROM api_tokens t JOIN users u ON u.id=t.user_id WHERE u.id=? AND u.account_type='bot' AND t.revoked_at IS NULL`, userID).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		return Created{}, ErrNotFound
	}
	if err != nil {
		return Created{}, err
	}
	if current != expectedTokenID {
		return Created{}, ErrBotTokenConflict
	}
	if _, err = tx.ExecContext(ctx, `UPDATE api_tokens SET revoked_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE id=?`, current); err != nil {
		return Created{}, err
	}
	created, err := createInTx(ctx, tx, userID, "MCP", []string{"cards:read", "cards:write"}, nil)
	if err != nil {
		return Created{}, err
	}
	if err = tx.Commit(); err != nil {
		return Created{}, err
	}
	return created, nil
}

// BotToken returns metadata only. HTTP authorization is enforced by the admin route.
func (s *Service) BotToken(ctx context.Context, userID string) (Token, error) {
	var kind string
	if err := s.db.QueryRowContext(ctx, `SELECT account_type FROM users WHERE id=?`, userID).Scan(&kind); errors.Is(err, sql.ErrNoRows) {
		return Token{}, ErrNotFound
	} else if err != nil {
		return Token{}, err
	}
	if kind != "bot" {
		return Token{}, ErrNotFound
	}
	items, err := s.List(ctx, userID)
	if err != nil {
		return Token{}, err
	}
	for _, item := range items {
		if item.RevokedAt == nil {
			return item, nil
		}
	}
	return Token{}, ErrNotFound
}
