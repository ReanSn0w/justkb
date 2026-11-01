package board

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/ReanSn0w/justkb/internal/auth"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestReferenceDataIntegration(t *testing.T) {
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
	if _, err := db.ExecContext(ctx, `TRUNCATE card_activity, api_tokens, web_sessions, attachments, card_tags, cards, group_members, users, user_groups, epics, tags RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	group, err := service.SaveGroup(ctx, "", "Backend", "#3366FF", nil)
	if err != nil {
		t.Fatal(err)
	}
	user, err := service.CreateUser(ctx, "developer", "Developer", "", "member", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.ResetUserPasswordByUsername(ctx, "DEVELOPER", "another correct password"); err != nil {
		t.Fatal(err)
	}
	if _, err := auth.NewService(db).Login(ctx, "developer", "another correct password", time.Hour); err != nil {
		t.Fatalf("login after password reset: %v", err)
	}
	admin, err := service.CreateUser(ctx, "admin", "Administrator", "", "admin", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SetUserActiveByUsername(ctx, admin.Username, false); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("disable last admin error=%v", err)
	}
	if _, err := service.CreateUser(ctx, "backup-admin", "Backup Administrator", "", "admin", "correct horse battery staple"); err != nil {
		t.Fatal(err)
	}
	disabled, err := service.SetUserActiveByUsername(ctx, admin.Username, false)
	if err != nil || disabled.IsActive {
		t.Fatalf("disabled admin=%#v err=%v", disabled, err)
	}
	loaded, err := service.UserByUsername(ctx, "ADMIN")
	if err != nil || loaded.ID != admin.ID || loaded.IsActive {
		t.Fatalf("loaded admin=%#v err=%v", loaded, err)
	}
	if err := service.SetUserGroups(ctx, user.ID, []string{group.ID}); err != nil {
		t.Fatal(err)
	}
	users, err := service.ListUsers(ctx, false, group.ID)
	if err != nil || len(users) != 1 || len(users[0].GroupIDs) != 1 {
		t.Fatalf("users = %#v, err = %v", users, err)
	}
	epic, err := service.SaveEpic(ctx, "", "Platform", "#AA5500", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := service.GetEpic(ctx, epic.ID)
	if err != nil || detail.Name != "Platform" || detail.CardCount != 0 {
		t.Fatalf("epic detail = %#v, err = %v", detail, err)
	}
	if _, err := service.SaveEpic(ctx, epic.ID, epic.Name, epic.Color, nil, true); err != nil {
		t.Fatal(err)
	}
	epics, err := service.ListEpics(ctx, false)
	if err != nil || len(epics) != 0 {
		t.Fatalf("active epics = %#v, err = %v", epics, err)
	}
	if _, err := service.SaveTag(ctx, "", "bug", "bad-color"); !errors.Is(err, ErrValidation) {
		t.Fatalf("invalid color error = %v", err)
	}
	columns, err := service.ListColumns(ctx)
	if err != nil || len(columns) < 2 {
		t.Fatalf("columns = %#v, err = %v", columns, err)
	}
	ids := make([]string, len(columns))
	for index := range columns {
		ids[len(columns)-1-index] = columns[index].ID
	}
	if err := service.ReorderColumns(ctx, ids); err != nil {
		t.Fatal(err)
	}
	reordered, _ := service.ListColumns(ctx)
	if reordered[0].ID != ids[0] {
		t.Fatalf("first column = %s, want %s", reordered[0].ID, ids[0])
	}
}
