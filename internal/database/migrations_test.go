package database

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestSQLiteMigrationsRoundTrip(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "jkb.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	const directory = "../../migrations"
	if err := goose.Up(db, directory); err != nil {
		t.Fatalf("up: %v", err)
	}
	version, err := goose.GetDBVersion(db)
	if err != nil {
		t.Fatal(err)
	}
	if version != 5 {
		t.Fatalf("version=%d", version)
	}
	if err := goose.DownTo(db, directory, 0); err != nil {
		t.Fatalf("down: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name='users'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("users table survived rollback")
	}
	if err := goose.Up(db, directory); err != nil {
		t.Fatalf("second up: %v", err)
	}
}

func TestAdditionalAssigneesMigrationPreservesLegacyCards(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "existing.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	const directory = "../../migrations"
	if err := goose.UpTo(db, directory, 4); err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`
		INSERT INTO users(id,username,display_name,password_hash,is_active)
		VALUES ('owner','owner','Owner','unused',0), ('extra','extra','Extra','unused',1);
		INSERT INTO cards(id,number,title,description,column_id,assignee_id,position,version,is_archived,created_by)
		VALUES ('existing',10,'Existing','Keep this',(SELECT id FROM board_columns LIMIT 1),'owner',2000,7,1,'owner');
		INSERT INTO card_activity(id,card_id,actor_user_id,action,changes)
		VALUES ('history','existing','owner','created','{}');
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(db, directory); err != nil {
		t.Fatal(err)
	}
	var preserved int
	err = db.QueryRow(`SELECT count(*) FROM cards c JOIN card_activity a ON a.card_id=c.id
		WHERE c.id='existing' AND c.assignee_id='owner' AND c.version=7 AND c.is_archived=1
		  AND c.description='Keep this' AND c.position=2000 AND a.id='history'`).Scan(&preserved)
	if err != nil || preserved != 1 {
		t.Fatalf("preserved=%d err=%v", preserved, err)
	}
	if _, err := db.Exec(`INSERT INTO card_additional_assignees VALUES ('existing','extra',1)`); err != nil {
		t.Fatal(err)
	}
	if err := goose.Down(db, directory); err == nil {
		t.Fatal("rollback would discard an additional assignee")
	}
	var additional int
	if err := db.QueryRow(`SELECT count(*) FROM card_additional_assignees`).Scan(&additional); err != nil || additional != 1 {
		t.Fatalf("additional=%d err=%v", additional, err)
	}
}
