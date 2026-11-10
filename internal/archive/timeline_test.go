package archive

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func TestTimelinePaginationGroupingAndHistory(t *testing.T) {
	db := fixture(t)
	ctx := context.Background()
	s := NewService(db)
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	for _, id := range []string{"a", "b", "c"} {
		if err := write(t, db, func(tx *sql.Tx) error { return Card(ctx, tx, Actor{UserID: "u"}, id, 1, true, "", now) }); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	cursor := ""
	for i := 0; i < 3; i++ {
		page, err := s.List(ctx, Filters{Limit: 1, Cursor: cursor})
		if err != nil || len(page.Items) != 1 {
			t.Fatalf("page=%+v err=%v", page, err)
		}
		id := page.Items[0].EntityID
		if seen[id] {
			t.Fatal("duplicate page item")
		}
		seen[id] = true
		cursor = page.NextCursor
	}
	if cursor != "" {
		t.Fatal("unexpected final cursor")
	}
	if err := write(t, db, func(tx *sql.Tx) error {
		return Card(ctx, tx, Actor{UserID: "u"}, "a", 2, false, "", now.Add(time.Hour))
	}); err != nil {
		t.Fatal(err)
	}
	page, err := s.List(ctx, Filters{})
	if err != nil || len(page.Items) != 2 {
		t.Fatalf("current=%+v err=%v", page, err)
	}
	page, err = s.List(ctx, Filters{History: true})
	if err != nil || len(page.Items) != 4 {
		t.Fatalf("history=%+v err=%v", page, err)
	}
	if err := write(t, db, func(tx *sql.Tx) error {
		return Project(ctx, tx, Actor{UserID: "u"}, "p", 1, true, "", now.Add(2*time.Hour))
	}); err != nil {
		t.Fatal(err)
	}
	page, err = s.List(ctx, Filters{})
	if err != nil || len(page.Items) != 3 {
		t.Fatalf("grouped=%+v err=%v", page, err)
	}
	if page.Items[0].EntityType != "project" || page.Items[0].ChildCount != 1 || page.Items[0].RestorableCards != 1 {
		t.Fatalf("batch=%+v", page.Items[0])
	}
	child, err := s.List(ctx, Filters{BatchID: *page.Items[0].BatchID})
	if err != nil || len(child.Items) != 1 || child.Items[0].EntityID != "a" {
		t.Fatalf("children=%+v err=%v", child, err)
	}
}
