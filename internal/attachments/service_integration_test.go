package attachments

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/ReanSn0w/justkb/internal/auth"
)

func TestAttachmentLifecycleIntegration(t *testing.T) {
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
	var columnID, cardID string
	if err := db.QueryRowContext(ctx, `SELECT id FROM board_columns ORDER BY position LIMIT 1`).Scan(&columnID); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, `INSERT INTO cards(title,column_id,position,created_by) VALUES('Attachment test',$1,1000,$2) RETURNING id`, columnID, user.ID).Scan(&cardID); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(db, t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := service.Upload(ctx, cardID, user.ID, "../notes.txt", bytes.NewBufferString("hello world"))
	if err != nil {
		t.Fatal(err)
	}
	if metadata.OriginalName != "notes.txt" {
		t.Fatalf("name=%q", metadata.OriginalName)
	}
	download, err := service.Open(ctx, metadata.ID)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(download.File)
	download.File.Close()
	if err != nil || string(payload) != "hello world" {
		t.Fatalf("payload=%q err=%v", payload, err)
	}
	if err := service.Delete(ctx, metadata.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Open(ctx, metadata.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("open after delete err=%v", err)
	}
	var epicID string
	if err := db.QueryRowContext(ctx, `INSERT INTO epics(name,color) VALUES('Files','#00d1b2') RETURNING id`).Scan(&epicID); err != nil {
		t.Fatal(err)
	}
	epicFile, err := service.UploadEpic(ctx, epicID, user.ID, "brief.txt", bytes.NewBufferString("epic brief"))
	if err != nil {
		t.Fatal(err)
	}
	if epicFile.EpicID == nil || *epicFile.EpicID != epicID || epicFile.CardID != nil {
		t.Fatalf("epic attachment owner=%#v", epicFile)
	}
	listed, err := service.ListEpic(ctx, epicID)
	if err != nil || len(listed) != 1 || listed[0].ID != epicFile.ID {
		t.Fatalf("epic attachments=%#v err=%v", listed, err)
	}
}
