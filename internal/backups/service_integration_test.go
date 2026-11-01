package backups

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/ReanSn0w/justkb/internal/board"
	"github.com/ReanSn0w/justkb/internal/maintenance"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestBackupRestoreIntegration(t *testing.T) {
	databaseURL := os.Getenv("JKB_BACKUP_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("JKB_BACKUP_TEST_DATABASE_URL is not set")
	}
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err := db.ExecContext(ctx, `
		TRUNCATE comments,card_activity,api_tokens,web_sessions,attachments,card_tags,cards,
		group_members,users,epics,tags,board_columns,user_groups RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	user, err := board.NewService(db).CreateUser(ctx, "backup-user", "Before backup", "", "admin", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	storage := filepath.Join(t.TempDir(), "storage")
	if err := os.MkdirAll(storage, 0o750); err != nil {
		t.Fatal(err)
	}
	storedFile := filepath.Join(storage, "attachment")
	if err := os.WriteFile(storedFile, []byte("before backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(db, databaseURL, storage, &maintenance.Gate{})
	if err != nil {
		t.Fatal(err)
	}
	archive, err := service.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if _, err := db.ExecContext(ctx, `UPDATE users SET display_name='After backup' WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storedFile, []byte("after backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(archive.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Restore(ctx, file); err != nil {
		file.Close()
		t.Fatal(err)
	}
	_ = file.Close()
	var displayName string
	if err := db.QueryRowContext(ctx, `SELECT display_name FROM users WHERE id=$1`, user.ID).Scan(&displayName); err != nil {
		t.Fatal(err)
	}
	if displayName != "Before backup" {
		t.Fatalf("display name after restore=%q", displayName)
	}
	content, err := os.ReadFile(storedFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "before backup" {
		t.Fatalf("file after restore=%q", content)
	}
}
