package attachments

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/ReanSn0w/justkb/internal/auth"
	"github.com/ReanSn0w/justkb/internal/board"
	"github.com/ReanSn0w/justkb/internal/cards"
	"github.com/ReanSn0w/justkb/internal/testdb"
)

func TestAttachmentLifecycleIntegration(t *testing.T) {
	db := testdb.Open(t).DB
	ctx := context.Background()
	user, err := auth.NewService(db).CreateAdmin(ctx, "admin", "Admin", "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	var columnID string
	if err := db.QueryRowContext(ctx, `SELECT id FROM board_columns ORDER BY position LIMIT 1`).Scan(&columnID); err != nil {
		t.Fatal(err)
	}
	card, err := cards.NewService(db).Create(ctx, cards.Actor{UserID: user.ID}, cards.CreateInput{Title: "Attachment test", ColumnID: columnID})
	if err != nil {
		t.Fatal(err)
	}
	cardID := card.ID
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
	epic, err := board.NewService(db).SaveEpic(ctx, "", "Files", "#00d1b2", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	epicID := epic.ID
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
