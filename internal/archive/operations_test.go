package archive

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	appdb "github.com/ReanSn0w/justkb/internal/database"
	"github.com/ReanSn0w/justkb/internal/testdb"
)

func fixture(t *testing.T) *sql.DB {
	t.Helper()
	db := testdb.Open(t).DB
	_, err := db.Exec(`INSERT INTO users(id,username,display_name,password_hash) VALUES('u','u','User','unused');
 INSERT INTO epics(id,name,color) VALUES('p','Project','#112233');
 INSERT INTO cards(id,number,title,column_id,epic_id,position,created_by,column_entered_at) VALUES
 ('a',1,'A',(SELECT done_column_id FROM archive_policy),'p',100,'u','2020-01-01'),
 ('b',2,'B',(SELECT done_column_id FROM archive_policy),'p',200,'u','2020-01-01'),
 ('c',3,'C',(SELECT done_column_id FROM archive_policy),'p',300,'u','2020-01-01');`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}
func write(t *testing.T, db *sql.DB, fn func(*sql.Tx) error) error {
	t.Helper()
	tx, err := appdb.BeginWrite(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
func TestProjectCascadeAndSelectiveRestoration(t *testing.T) {
	for _, mode := range []string{"project_only", "with_cards"} {
		t.Run(mode, func(t *testing.T) {
			db := fixture(t)
			ctx := context.Background()
			actor := Actor{UserID: "u"}
			now := time.Now().UTC()
			if err := write(t, db, func(tx *sql.Tx) error { return Card(ctx, tx, actor, "a", 1, true, "", now) }); err != nil {
				t.Fatal(err)
			}
			if err := write(t, db, func(tx *sql.Tx) error { return Project(ctx, tx, actor, "p", 1, true, "", now) }); err != nil {
				t.Fatal(err)
			}
			if err := write(t, db, func(tx *sql.Tx) error { return Card(ctx, tx, actor, "b", 2, false, "", now) }); !errors.Is(err, ErrArchivedProject) {
				t.Fatalf("restore under archived parent: %v", err)
			}
			if err := write(t, db, func(tx *sql.Tx) error { return Project(ctx, tx, actor, "p", 2, false, mode, now.Add(time.Hour)) }); err != nil {
				t.Fatal(err)
			}
			var active int
			if err := db.QueryRow(`SELECT count(*) FROM cards WHERE NOT is_archived`).Scan(&active); err != nil {
				t.Fatal(err)
			}
			want := 0
			if mode == "with_cards" {
				want = 2
			}
			if active != want {
				t.Fatalf("active=%d want=%d", active, want)
			}
			var n int
			if err := db.QueryRow(`SELECT count(*) FROM card_activity WHERE action='archived'`).Scan(&n); err != nil || n != 3 {
				t.Fatalf("activity=%d err=%v", n, err)
			}
		})
	}
}
func TestArchiveIsAtomicIdempotentAndSystemAttributed(t *testing.T) {
	db := fixture(t)
	ctx := context.Background()
	now := time.Now().UTC()
	actor := Actor{Automatic: true, RuleDays: 3}
	if err := write(t, db, func(tx *sql.Tx) error { return Project(ctx, tx, actor, "p", 1, true, "", now) }); err != nil {
		t.Fatal(err)
	}
	if err := write(t, db, func(tx *sql.Tx) error { return Project(ctx, tx, actor, "p", 2, true, "", now) }); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM archive_events WHERE actor_type='system' AND actor_user_id IS NULL`).Scan(&n); err != nil || n != 4 {
		t.Fatalf("events=%d err=%v", n, err)
	}
	if _, err := db.Exec(`INSERT INTO epics(id,name,color) VALUES('duplicate','Project','#112233')`); err != nil {
		t.Fatal(err)
	}
	if err := write(t, db, func(tx *sql.Tx) error { return Project(ctx, tx, Actor{UserID: "u"}, "p", 2, false, "with_cards", now) }); err == nil {
		t.Fatal("name collision accepted")
	}
	if err := db.QueryRow(`SELECT count(*) FROM cards WHERE NOT is_archived`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("partial restoration=%d err=%v", n, err)
	}
}
