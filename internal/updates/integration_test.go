package updates_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/ReanSn0w/justkb/internal/archive"
	"github.com/ReanSn0w/justkb/internal/attachments"
	"github.com/ReanSn0w/justkb/internal/board"
	"github.com/ReanSn0w/justkb/internal/cards"
	"github.com/ReanSn0w/justkb/internal/comments"
	"github.com/ReanSn0w/justkb/internal/maintenance"
	"github.com/ReanSn0w/justkb/internal/testdb"
	"github.com/ReanSn0w/justkb/internal/updates"
)

func fixture(t *testing.T) (*sql.DB, string) {
	t.Helper()
	db := testdb.Open(t).DB
	if _, err := db.Exec(`INSERT INTO users(id,username,display_name,password_hash) VALUES('u','u','User','unused'),('v','v','Other','unused');INSERT INTO epics(id,name,color) VALUES('a','A','#112233'),('b','B','#112233')`); err != nil {
		t.Fatal(err)
	}
	var col string
	if err := db.QueryRow(`SELECT id FROM board_columns ORDER BY position LIMIT 1`).Scan(&col); err != nil {
		t.Fatal(err)
	}
	return db, col
}
func feed(t *testing.T, db *sql.DB, p string) []updates.Event {
	t.Helper()
	page, err := updates.NewService(db).List(context.Background(), p, "", 500)
	if err != nil {
		t.Fatal(err)
	}
	return page.Items
}
func TestProjectHistoryTracksOwnershipAndFields(t *testing.T) {
	db, col := fixture(t)
	ctx := context.Background()
	s := cards.NewService(db)
	actor := cards.Actor{UserID: "u"}
	card, err := s.Create(ctx, actor, cards.CreateInput{Title: "First", ColumnID: col, EpicID: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = comments.NewService(db).CreateCard(ctx, card.ID, comments.Actor{UserID: "v"}, "Before transfer"); err != nil {
		t.Fatal(err)
	}
	next := "b"
	title := "Changed"
	card, err = s.Update(ctx, actor, card.ID, cards.UpdateInput{ExpectedVersion: card.Version, EpicID: &next, Title: &title, AssigneeIDs: []string{"u", "v"}})
	if err != nil {
		t.Fatal(err)
	}
	a, b := feed(t, db, "a"), feed(t, db, "b")
	if len(a) != 3 || len(b) != 1 || a[2].ID != b[0].ID || len(b[0].ProjectIDs) != 2 {
		t.Fatalf("transfer: %+v %+v", a, b)
	}
	for _, field := range []string{"title", "epic_id", "assignee_ids"} {
		if _, ok := b[0].Changes[field]; !ok {
			t.Fatal("missing field", field)
		}
	}
	if a[1].Type != "comment.created" || a[1].ActorUserID == nil || *a[1].ActorUserID != "v" {
		t.Fatalf("comment: %+v", a[1])
	}
	card, err = s.Update(ctx, actor, card.ID, cards.UpdateInput{ExpectedVersion: card.Version, Title: &title})
	if err != nil {
		t.Fatal(err)
	}
	if len(feed(t, db, "b")) != 1 {
		t.Fatal("no-op emitted an update")
	}
	if _, err = s.Update(ctx, actor, card.ID, cards.UpdateInput{ExpectedVersion: card.Version - 1, Title: &title}); !errors.Is(err, cards.ErrVersionConflict) {
		t.Fatal(err)
	}
	empty := ""
	card, err = s.Update(ctx, actor, card.ID, cards.UpdateInput{ExpectedVersion: card.Version, EpicID: &empty})
	if err != nil {
		t.Fatal(err)
	}
	if len(feed(t, db, "b")) != 2 {
		t.Fatal("removal missing")
	}
	card, err = s.Update(ctx, actor, card.ID, cards.UpdateInput{ExpectedVersion: card.Version, EpicID: &next})
	if err != nil {
		t.Fatal(err)
	}
	if len(feed(t, db, "b")) != 3 {
		t.Fatal("arrival missing")
	}
	// Changing columns is a real movement.
	var target string
	if err = db.QueryRow(`SELECT id FROM board_columns WHERE id<>? LIMIT 1`, col).Scan(&target); err != nil {
		t.Fatal(err)
	}
	card, err = s.Move(ctx, actor, card.ID, cards.MoveInput{ExpectedVersion: card.Version, TargetColumnID: target})
	if err != nil {
		t.Fatal(err)
	}
	events := feed(t, db, "b")
	if events[len(events)-1].Type != "card.moved" {
		t.Fatal("movement missing")
	}
}
func TestProjectArchiveAndRelatedEvents(t *testing.T) {
	db, col := fixture(t)
	ctx := context.Background()
	bs := board.NewService(db)
	cs := cards.NewService(db)
	p, err := bs.SaveEpic(ctx, "", "Created", "#112233", nil, false, board.EpicSaveOptions{Actor: archive.Actor{UserID: "u"}})
	if err != nil {
		t.Fatal(err)
	}
	card, err := cs.Create(ctx, cards.Actor{UserID: "u"}, cards.CreateInput{Title: "Card", ColumnID: col, EpicID: p.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = comments.NewService(db).CreateEpic(ctx, p.ID, comments.Actor{UserID: "u"}, "Project note"); err != nil {
		t.Fatal(err)
	}
	fs, err := attachments.NewService(db, t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	for _, projectFile := range []bool{false, true} {
		var f attachments.Metadata
		if projectFile {
			f, err = fs.UploadEpic(ctx, p.ID, "u", "note.txt", strings.NewReader("hello"))
		} else {
			f, err = fs.Upload(ctx, card.ID, "u", "note.txt", strings.NewReader("hello"))
		}
		if err != nil {
			t.Fatal(err)
		}
		if err = fs.Delete(ctx, f.ID, updates.Actor{UserID: "v"}); err != nil {
			t.Fatal(err)
		}
	}
	p, err = bs.SaveEpic(ctx, p.ID, "Renamed", p.Color, nil, false, board.EpicSaveOptions{ExpectedVersion: p.Version, Actor: archive.Actor{UserID: "u"}})
	if err != nil {
		t.Fatal(err)
	}
	p, err = bs.SaveEpic(ctx, p.ID, p.Name, p.Color, nil, true, board.EpicSaveOptions{ExpectedVersion: p.Version, Actor: archive.Actor{UserID: "u"}})
	if err != nil {
		t.Fatal(err)
	}
	events := feed(t, db, p.ID)
	n := len(events)
	if events[n-2].Type != "card.archived" || events[n-1].Type != "project.archived" {
		t.Fatalf("archive events: %+v", events)
	}
	if events[n-2].Changes["batch_id"] != events[n-1].Changes["batch_id"] {
		t.Fatal("batch mismatch")
	}
	p, err = bs.SaveEpic(ctx, p.ID, p.Name, p.Color, nil, false, board.EpicSaveOptions{ExpectedVersion: p.Version, Actor: archive.Actor{UserID: "u"}, RestoreMode: "with_cards"})
	if err != nil {
		t.Fatal(err)
	}
	events = feed(t, db, p.ID)
	if len(events) != n+2 || events[n].Type != "card.restored" || events[n+1].Type != "project.restored" {
		t.Fatalf("restore: %+v", events)
	}
	for _, e := range events {
		if e.ActorType != "user" || e.ActorUserID == nil {
			t.Fatalf("actor: %+v", e)
		}
	}
}
func TestEventFailureRollsBackMutationsAndFiles(t *testing.T) {
	db, col := fixture(t)
	ctx := context.Background()
	cs := cards.NewService(db)
	card, err := cs.Create(ctx, cards.Actor{UserID: "u"}, cards.CreateInput{Title: "Original", ColumnID: col, EpicID: "a"})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	fs, err := attachments.NewService(db, root, 1024)
	if err != nil {
		t.Fatal(err)
	}
	file, err := fs.Upload(ctx, card.ID, "u", "keep.txt", strings.NewReader("keep"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`CREATE TRIGGER reject_update_event BEFORE INSERT ON project_updates BEGIN SELECT RAISE(ABORT,'injected failure'); END`); err != nil {
		t.Fatal(err)
	}
	title := "Lost"
	if _, err = cs.Update(ctx, cards.Actor{UserID: "u"}, card.ID, cards.UpdateInput{ExpectedVersion: card.Version, Title: &title}); err == nil {
		t.Fatal("update succeeded")
	}
	if _, err = cs.Create(ctx, cards.Actor{UserID: "u"}, cards.CreateInput{Title: "Lost", ColumnID: col, EpicID: "a"}); err == nil {
		t.Fatal("create succeeded")
	}
	if _, err = comments.NewService(db).CreateCard(ctx, card.ID, comments.Actor{UserID: "u"}, "Lost"); err == nil {
		t.Fatal("comment succeeded")
	}
	if _, err = fs.Upload(ctx, card.ID, "u", "lost.txt", strings.NewReader("lost")); err == nil {
		t.Fatal("upload succeeded")
	}
	if err = fs.Delete(ctx, file.ID, updates.Actor{UserID: "u"}); err == nil {
		t.Fatal("delete succeeded")
	}
	if _, err = board.NewService(db).SaveEpic(ctx, "", "Lost", "#112233", nil, false); err == nil {
		t.Fatal("project creation succeeded")
	}
	if _, err = board.NewService(db).SaveEpic(ctx, "a", "A", "#112233", nil, true, board.EpicSaveOptions{ExpectedVersion: 1, Actor: archive.Actor{UserID: "u"}}); err == nil {
		t.Fatal("archive succeeded")
	}
	current, err := cs.Get(ctx, card.ID)
	if err != nil || current.Title != "Original" || current.Version != 1 || current.IsArchived {
		t.Fatalf("rollback: %+v %v", current, err)
	}
	var n int
	if err = db.QueryRow(`SELECT count(*) FROM card_activity`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("activity: %d %v", n, err)
	}
	if err = db.QueryRow(`SELECT count(*) FROM comments`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("comments: %d %v", n, err)
	}
	files, err := os.ReadDir(root)
	if err != nil || len(files) != 1 || strings.HasSuffix(files[0].Name(), ".deleting") {
		t.Fatalf("files: %v %v", files, err)
	}
	download, err := fs.Open(ctx, file.ID)
	if err != nil {
		t.Fatal(err)
	}
	download.File.Close()
	if len(feed(t, db, "a")) != 2 {
		t.Fatal("failed mutations emitted events")
	}
}
func TestAutomaticArchivePublishesSystemEvent(t *testing.T) {
	db, _ := fixture(t)
	ctx := context.Background()
	var done string
	if err := db.QueryRow(`SELECT done_column_id FROM archive_policy`).Scan(&done); err != nil {
		t.Fatal(err)
	}
	card, err := cards.NewService(db).Create(ctx, cards.Actor{UserID: "u"}, cards.CreateInput{Title: "Done", ColumnID: done, EpicID: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`UPDATE archive_policy SET cards_enabled=1,projects_enabled=0; UPDATE cards SET column_entered_at='2020-01-01' WHERE id=?`, card.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = archive.NewService(db).RunOnce(ctx, &maintenance.Gate{}); err != nil {
		t.Fatal(err)
	}
	events := feed(t, db, "a")
	if len(events) != 2 || events[1].Type != "card.archived" || events[1].ActorType != "system" || events[1].ActorUserID != nil || events[1].Changes["reason"] != "automatic" {
		t.Fatalf("system archive: %+v", events)
	}
}
