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
	if version != 4 {
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
