package updates

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	appdb "github.com/ReanSn0w/justkb/internal/database"
	"github.com/ReanSn0w/justkb/internal/testdb"
	"testing"
	"time"
)

func TestPollingCursor(t *testing.T) {
	db := testdb.Open(t).DB
	ctx := context.Background()
	s := NewService(db)
	if _, err := db.Exec(`INSERT INTO epics(id,name,color) VALUES('a','A','#112233'),('b','B','#112233')`); err != nil {
		t.Fatal(err)
	}
	appendEvent := func(projects ...string) {
		t.Helper()
		tx, err := appdb.BeginWrite(ctx, db)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
		err = Append(ctx, tx, Actor{}, Event{Type: "card.updated", EntityType: "card", EntityID: "c", ProjectIDs: projects, OccurredAt: time.Unix(1, 0).In(time.FixedZone("local", 3*60*60)), Changes: map[string]any{}})
		if err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(); err != nil {
			t.Fatal(err)
		}
	}
	initial, err := s.List(ctx, "a", "now", 1)
	if err != nil || initial.NextCursor == "" || len(initial.Items) != 0 {
		t.Fatalf("initial: %+v %v", initial, err)
	}
	appendEvent("a")
	appendEvent("b")
	appendEvent("a", "b", "a")
	first, err := s.List(ctx, "a", initial.NextCursor, 1)
	if err != nil || len(first.Items) != 1 || !first.HasMore {
		t.Fatalf("first: %+v %v", first, err)
	}
	if first.Items[0].OccurredAt.Location() != time.UTC {
		t.Fatal("event timestamp is not UTC")
	}
	appendEvent("a") // committed between pages, with the same timestamp
	next, err := s.List(ctx, "a", first.NextCursor, 1)
	if err != nil || len(next.Items) != 1 || !next.HasMore || len(next.Items[0].ProjectIDs) != 2 {
		t.Fatalf("next: %+v %v", next, err)
	}
	repeated, err := s.List(ctx, "a", first.NextCursor, 1)
	if err != nil || repeated.Items[0].ID != next.Items[0].ID {
		t.Fatal("retry changed event", err)
	}
	last, err := s.List(ctx, "a", next.NextCursor, 1)
	if err != nil || len(last.Items) != 1 || last.HasMore {
		t.Fatalf("last: %+v %v", last, err)
	}
	empty, err := s.List(ctx, "a", last.NextCursor, 1)
	if err != nil || len(empty.Items) != 0 || empty.NextCursor == "" {
		t.Fatalf("empty: %+v %v", empty, err)
	}
	appendEvent("b")
	empty2, err := s.List(ctx, "a", empty.NextCursor, 1)
	if err != nil || len(empty2.Items) != 0 || empty2.NextCursor == empty.NextCursor {
		t.Fatal("empty high water mark", err)
	}
	for _, v := range []string{"bad", first.NextCursor} {
		p := "a"
		if v == first.NextCursor {
			p = "b"
		}
		if _, err = s.List(ctx, p, v, 1); !errors.Is(err, ErrCursor) {
			t.Fatalf("cursor: %v", err)
		}
	}
	raw, _ := base64.RawURLEncoding.DecodeString(first.NextCursor)
	var c cursor
	json.Unmarshal(raw, &c)
	c.Sequence = 10000
	raw, _ = json.Marshal(c)
	if _, err = s.List(ctx, "a", base64.RawURLEncoding.EncodeToString(raw), 1); !errors.Is(err, ErrReset) {
		t.Fatal("future cursor", err)
	}
	if _, err = db.Exec(`UPDATE project_update_generation SET generation='new'`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.List(ctx, "a", first.NextCursor, 1); !errors.Is(err, ErrReset) {
		t.Fatal("generation", err)
	}
	if _, err = s.List(ctx, "missing", "", 1); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err = s.List(ctx, "a", "", 501); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
}
func TestAppendRollback(t *testing.T) {
	db := testdb.Open(t).DB
	ctx := context.Background()
	tx, err := appdb.BeginWrite(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if err = Append(ctx, tx, Actor{}, Event{ProjectIDs: []string{"a"}}); err != nil {
		t.Fatal(err)
	}
	tx.Rollback()
	var count int
	if err = db.QueryRow(`SELECT count(*) FROM project_updates`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("rollback: %d %v", count, err)
	}
}

func TestPollingDuringConcurrentCommits(t *testing.T) {
	db := testdb.Open(t).DB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := db.Exec(`INSERT INTO epics(id,name,color) VALUES('p','Concurrent','#112233')`); err != nil {
		t.Fatal(err)
	}
	s := NewService(db)
	initial, err := s.List(ctx, "p", "now", 3)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		for range 40 {
			tx, err := appdb.BeginWrite(ctx, db)
			if err != nil {
				done <- err
				return
			}
			err = Append(ctx, tx, Actor{}, Event{ProjectIDs: []string{"p"}, EntityType: "card", EntityID: "c", Type: "card.updated"})
			if err != nil {
				tx.Rollback()
				done <- err
				return
			}
			if err = tx.Commit(); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	seen := map[string]bool{}
	cursor := initial.NextCursor
	for len(seen) < 40 {
		page, err := s.List(ctx, "p", cursor, 3)
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range page.Items {
			if seen[event.ID] {
				t.Fatal("duplicate across committed cursors")
			}
			seen[event.ID] = true
		}
		cursor = page.NextCursor
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	page, err := s.List(ctx, "p", cursor, 3)
	if err != nil || len(page.Items) != 0 {
		t.Fatalf("final page: %+v %v", page, err)
	}
}
