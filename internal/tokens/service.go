package tokens

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ReanSn0w/justkb/internal/auth"
	appdb "github.com/ReanSn0w/justkb/internal/database"
	"github.com/ReanSn0w/justkb/internal/ids"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrInvalidScope = errors.New("invalid scope")
	ErrNotFound     = errors.New("token not found")
)

type Token struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"token_prefix"`
	Scopes     []string   `json:"scopes"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

type Created struct {
	Token
	Secret string `json:"token"`
}

type Service struct {
	db  *sql.DB
	now func() time.Time
}

func NewService(db *sql.DB) *Service { return &Service{db: db, now: time.Now} }

func (s *Service) Create(ctx context.Context, userID, name string, scopes []string, expiresAt *time.Time) (Created, error) {
	name = strings.TrimSpace(name)
	validatedScopes, err := validateScopes(scopes)
	if err != nil {
		return Created{}, err
	}
	if name == "" || (expiresAt != nil && !expiresAt.After(s.now())) {
		return Created{}, ErrInvalidToken
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Created{}, fmt.Errorf("generate API token: %w", err)
	}
	secret := "jkb_pat_" + base64.RawURLEncoding.EncodeToString(raw)
	digest := sha256.Sum256([]byte(secret))
	prefix := secret
	if len(prefix) > 20 {
		prefix = prefix[:20]
	}
	tx, err := appdb.BeginWrite(ctx, s.db)
	if err != nil {
		return Created{}, err
	}
	defer tx.Rollback()
	var token Token
	err = tx.QueryRowContext(ctx, `
		INSERT INTO api_tokens(id,user_id,name,token_prefix,token_digest,expires_at)
		VALUES($1,$2,$3,$4,$5,$6)
		RETURNING id,name,token_prefix,expires_at,last_used_at,revoked_at,created_at`,
		ids.New(), userID, name, prefix, digest[:], expiresAt,
	).Scan(&token.ID, &token.Name, &token.Prefix, &token.ExpiresAt, &token.LastUsedAt, &token.RevokedAt, &token.CreatedAt)
	if err != nil {
		return Created{}, err
	}
	for _, scope := range validatedScopes {
		if _, err := tx.ExecContext(ctx, `INSERT INTO api_token_scopes(token_id,scope) VALUES($1,$2)`, token.ID, scope); err != nil {
			return Created{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Created{}, err
	}
	token.Scopes = validatedScopes
	return Created{Token: token, Secret: secret}, nil
}

func (s *Service) List(ctx context.Context, userID string) ([]Token, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,name,token_prefix,expires_at,last_used_at,revoked_at,created_at FROM api_tokens WHERE user_id=$1 ORDER BY created_at DESC,id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Token, 0)
	for rows.Next() {
		var item Token
		if err := rows.Scan(&item.ID, &item.Name, &item.Prefix, &item.ExpiresAt, &item.LastUsedAt, &item.RevokedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Scopes, err = s.scopes(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) Revoke(ctx context.Context, userID, tokenID string) error {
	result, err := s.db.ExecContext(ctx, `UPDATE api_tokens SET revoked_at=COALESCE(revoked_at,CURRENT_TIMESTAMP),updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND user_id=$2`, tokenID, userID)
	if err != nil {
		return err
	}
	if count, _ := result.RowsAffected(); count == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) Authenticate(ctx context.Context, secret string) (auth.Principal, error) {
	if !strings.HasPrefix(secret, "jkb_pat_") {
		return auth.Principal{}, ErrInvalidToken
	}
	digest := sha256.Sum256([]byte(secret))
	var principal auth.Principal
	var scopes []string
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id,u.username,u.display_name,u.email,u.avatar_url,u.role,u.is_active,t.id
		FROM api_tokens t JOIN users u ON u.id=t.user_id
		WHERE t.token_digest=$1 AND t.revoked_at IS NULL AND (t.expires_at IS NULL OR t.expires_at>$2) AND u.is_active`,
		digest[:], s.now(),
	).Scan(&principal.User.ID, &principal.User.Username, &principal.User.DisplayName, &principal.User.Email, &principal.User.AvatarURL, &principal.User.Role, &principal.User.IsActive, &principal.TokenID)
	if errors.Is(err, sql.ErrNoRows) {
		return auth.Principal{}, ErrInvalidToken
	}
	if err != nil {
		return auth.Principal{}, err
	}
	scopes, err = s.scopes(ctx, principal.TokenID)
	if err != nil {
		return auth.Principal{}, err
	}
	principal.Scopes = make(map[string]bool, len(scopes))
	for _, scope := range scopes {
		principal.Scopes[scope] = true
	}
	s.touch(principal.TokenID)
	return principal, nil
}

func (s *Service) touch(id string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = s.db.ExecContext(ctx, `UPDATE api_tokens SET last_used_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE id=$1 AND (last_used_at IS NULL OR last_used_at<datetime(CURRENT_TIMESTAMP,'-5 minutes'))`, id)
	}()
}

func (s *Service) scopes(ctx context.Context, tokenID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT scope FROM api_token_scopes WHERE token_id=$1 ORDER BY scope`, tokenID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	scopes := make([]string, 0)
	for rows.Next() {
		var scope string
		if err := rows.Scan(&scope); err != nil {
			return nil, err
		}
		scopes = append(scopes, scope)
	}
	return scopes, rows.Err()
}

func validateScopes(scopes []string) ([]string, error) {
	seen := make(map[string]bool)
	result := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if scope != "cards:read" && scope != "cards:write" {
			return nil, ErrInvalidScope
		}
		if !seen[scope] {
			seen[scope] = true
			result = append(result, scope)
		}
	}
	if len(result) == 0 {
		return nil, ErrInvalidScope
	}
	sort.Strings(result)
	return result, nil
}
