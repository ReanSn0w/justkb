package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrForbidden          = errors.New("forbidden")
)

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

type Principal struct {
	User      User
	SessionID string
	CSRFToken string
	TokenID   string
	Scopes    map[string]bool
}

type Session struct {
	Token string
	CSRF  string
	User  User
}

type Service struct {
	db  *sql.DB
	now func() time.Time
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db, now: time.Now}
}

func (s *Service) Login(ctx context.Context, username, password string, ttl time.Duration) (Session, error) {
	var user User
	var passwordHash string
	err := s.db.QueryRowContext(ctx, `
		SELECT id, username, display_name, email, avatar_url, role::text, is_active, password_hash
		FROM users WHERE lower(username) = lower($1)`, username,
	).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Email, &user.AvatarURL, &user.Role, &user.IsActive, &passwordHash)
	if err != nil || !user.IsActive || !VerifyPassword(passwordHash, password) {
		return Session{}, ErrInvalidCredentials
	}
	user.GroupIDs, err = s.groupIDs(ctx, user.ID)
	if err != nil {
		return Session{}, fmt.Errorf("load user groups: %w", err)
	}
	token, tokenDigest, err := randomToken()
	if err != nil {
		return Session{}, err
	}
	csrf, csrfSecret, err := randomCSRF()
	if err != nil {
		return Session{}, err
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO web_sessions (user_id, token_digest, csrf_digest, expires_at)
		VALUES ($1, $2, $3, $4)`, user.ID, tokenDigest[:], csrfSecret, s.now().Add(ttl))
	if err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}
	return Session{Token: token, CSRF: csrf, User: user}, nil
}

func (s *Service) AuthenticateSession(ctx context.Context, token string) (Principal, error) {
	digest := sha256.Sum256([]byte(token))
	var principal Principal
	var csrfDigest []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT u.id, u.username, u.display_name, u.email, u.avatar_url, u.role::text, u.is_active,
		       s.id, s.csrf_digest
		FROM web_sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_digest = $1 AND s.expires_at > $2 AND u.is_active`, digest[:], s.now(),
	).Scan(&principal.User.ID, &principal.User.Username, &principal.User.DisplayName,
		&principal.User.Email, &principal.User.AvatarURL, &principal.User.Role, &principal.User.IsActive,
		&principal.SessionID, &csrfDigest)
	if errors.Is(err, sql.ErrNoRows) {
		return Principal{}, ErrUnauthenticated
	}
	if err != nil {
		return Principal{}, fmt.Errorf("authenticate session: %w", err)
	}
	principal.User.GroupIDs, err = s.groupIDs(ctx, principal.User.ID)
	if err != nil {
		return Principal{}, fmt.Errorf("load user groups: %w", err)
	}
	principal.CSRFToken = base64.RawURLEncoding.EncodeToString(csrfDigest)
	return principal, nil
}

func (s *Service) ValidateCSRF(ctx context.Context, sessionID, provided string) bool {
	providedSecret, err := base64.RawURLEncoding.DecodeString(provided)
	if err != nil {
		return false
	}
	var expected []byte
	if err := s.db.QueryRowContext(ctx, `SELECT csrf_digest FROM web_sessions WHERE id = $1`, sessionID).Scan(&expected); err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(providedSecret, expected) == 1
}

func (s *Service) Logout(ctx context.Context, token string) error {
	digest := sha256.Sum256([]byte(token))
	_, err := s.db.ExecContext(ctx, `DELETE FROM web_sessions WHERE token_digest = $1`, digest[:])
	return err
}

func (s *Service) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	var encoded string
	if err := s.db.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = $1 AND is_active`, userID).Scan(&encoded); err != nil {
		return ErrInvalidCredentials
	}
	if !VerifyPassword(encoded, currentPassword) {
		return ErrInvalidCredentials
	}
	nextHash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, userID, nextHash); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM web_sessions WHERE user_id = $1`, userID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Service) CreateAdmin(ctx context.Context, username, displayName, password string) (User, error) {
	encoded, err := HashPassword(password)
	if err != nil {
		return User{}, err
	}
	var user User
	err = s.db.QueryRowContext(ctx, `
		INSERT INTO users (username, display_name, role, password_hash)
		VALUES ($1, $2, 'admin', $3)
		RETURNING id, username, display_name, email, avatar_url, role::text, is_active`, username, displayName, encoded,
	).Scan(&user.ID, &user.Username, &user.DisplayName, &user.Email, &user.AvatarURL, &user.Role, &user.IsActive)
	return user, err
}

func (s *Service) groupIDs(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT group_id FROM group_members WHERE user_id = $1 ORDER BY group_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		groups = append(groups, id)
	}
	return groups, rows.Err()
}

func randomToken() (string, [32]byte, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", [32]byte{}, fmt.Errorf("generate token: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	return token, sha256.Sum256([]byte(token)), nil
}

func randomCSRF() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate CSRF token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), raw, nil
}
