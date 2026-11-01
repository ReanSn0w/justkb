package tokens

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/ReanSn0w/justkb/internal/auth"
)

func TestTokenLifecycleIntegration(t *testing.T) {
	url := os.Getenv("JKB_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("JKB_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("pgx", url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `TRUNCATE card_activity,api_tokens,web_sessions,attachments,card_tags,cards,group_members,users,user_groups,epics,tags RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	user, err := auth.NewService(db).CreateAdmin(ctx, "admin", "Admin", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	expires := time.Now().Add(time.Hour)
	created, err := service.Create(ctx, user.ID, "agent", []string{"cards:write", "cards:read", "cards:read"}, &expires)
	if err != nil {
		t.Fatal(err)
	}
	if created.Secret == "" || len(created.Scopes) != 2 {
		t.Fatalf("created=%#v", created)
	}
	listed, err := service.List(ctx, user.ID)
	if err != nil || len(listed) != 1 {
		t.Fatalf("listed=%#v err=%v", listed, err)
	}
	principal, err := service.Authenticate(ctx, created.Secret)
	if err != nil || !principal.Scopes["cards:read"] {
		t.Fatalf("principal=%#v err=%v", principal, err)
	}
	if err := service.Revoke(ctx, user.ID, created.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Authenticate(ctx, created.Secret); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("auth after revoke=%v", err)
	}
	if _, err := service.Create(ctx, user.ID, "bad", []string{"admin"}, nil); !errors.Is(err, ErrInvalidScope) {
		t.Fatalf("scope error=%v", err)
	}
}
