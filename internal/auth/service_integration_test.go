package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ReanSn0w/justkb/internal/testdb"
)

func TestSessionLifecycleIntegration(t *testing.T) {
	db := testdb.Open(t).DB
	ctx := context.Background()
	service := NewService(db)
	user, err := service.CreateAdmin(ctx, "admin", "Administrator", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	session, err := service.Login(ctx, "ADMIN", "correct horse battery staple", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	principal, err := service.AuthenticateSession(ctx, session.Token)
	if err != nil {
		t.Fatal(err)
	}
	if principal.User.ID != user.ID || principal.User.Role != "admin" {
		t.Fatalf("unexpected principal: %#v", principal)
	}
	if !service.ValidateCSRF(ctx, principal.SessionID, session.CSRF) {
		t.Fatal("valid CSRF token rejected")
	}
	if service.ValidateCSRF(ctx, principal.SessionID, "invalid") {
		t.Fatal("invalid CSRF token accepted")
	}
	if _, err := db.ExecContext(ctx, `UPDATE users SET is_active = false WHERE id = $1`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.AuthenticateSession(ctx, session.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("inactive user auth error = %v", err)
	}
}
