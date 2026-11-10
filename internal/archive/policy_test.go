package archive

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestPolicyPreviewAndColumnReset(t *testing.T) {
	db := fixture(t)
	ctx := context.Background()
	s := NewService(db)
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	if err := write(t, db, func(tx *sql.Tx) error { return RefreshProjects(ctx, tx, now.Add(-72*time.Hour), "p") }); err != nil {
		t.Fatal(err)
	}
	p, err := s.Policy(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p.CardsEnabled = true
	p.ProjectsEnabled = true
	preview, err := s.Preview(ctx, p)
	if err != nil || preview.Cards != 3 || preview.Projects != 1 {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	saved, err := s.SavePolicy(ctx, p, p.Version)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SavePolicy(ctx, p, p.Version); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale settings: %v", err)
	}
	var other string
	if err := db.QueryRow(`SELECT id FROM board_columns WHERE id<>$1 LIMIT 1`, *p.DoneColumnID).Scan(&other); err != nil {
		t.Fatal(err)
	}
	saved.DoneColumnID = &other
	preview, err = s.Preview(ctx, saved)
	if err != nil || preview.Cards != 0 || preview.Projects != 0 {
		t.Fatalf("changed column preview=%+v err=%v", preview, err)
	}
	if _, err = s.SavePolicy(ctx, saved, saved.Version); err != nil {
		t.Fatal(err)
	}
	var since sql.NullTime
	if err := db.QueryRow(`SELECT completed_since FROM epics WHERE id='p'`).Scan(&since); err != nil || since.Valid {
		t.Fatalf("completion=%+v err=%v", since, err)
	}
}
func TestCompletionPreservesIntervalAndCountsArchivedUnfinished(t *testing.T) {
	db := fixture(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	refresh := func(at time.Time) {
		t.Helper()
		if err := write(t, db, func(tx *sql.Tx) error { return RefreshProjects(ctx, tx, at, "p") }); err != nil {
			t.Fatal(err)
		}
	}
	refresh(now)
	refresh(now.Add(time.Hour))
	var since time.Time
	if err := db.QueryRow(`SELECT completed_since FROM epics WHERE id='p'`).Scan(&since); err != nil || !since.Equal(now) {
		t.Fatalf("since=%v err=%v", since, err)
	}
	if _, err := db.Exec(`UPDATE cards SET is_archived=1,column_id=(SELECT id FROM board_columns ORDER BY position LIMIT 1) WHERE id='a'`); err != nil {
		t.Fatal(err)
	}
	refresh(now.Add(time.Hour))
	var value sql.NullTime
	if err := db.QueryRow(`SELECT completed_since FROM epics WHERE id='p'`).Scan(&value); err != nil || value.Valid {
		t.Fatalf("archived unfinished counted as done: %v %v", value, err)
	}
}
