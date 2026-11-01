package testdb

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	appdb "github.com/ReanSn0w/justkb/internal/database"
	"github.com/pressly/goose/v3"
)

type Database struct {
	DB   *sql.DB
	Path string
}

// Open creates an isolated migrated SQLite database for one test.
func Open(t testing.TB) Database {
	t.Helper()
	path := filepath.Join(t.TempDir(), "jkb.sqlite")
	db, err := appdb.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test database helper path")
	}
	migrations := filepath.Join(filepath.Dir(source), "..", "..", "migrations")
	if err := goose.Up(db, migrations); err != nil {
		t.Fatal(err)
	}
	return Database{DB: db, Path: path}
}
