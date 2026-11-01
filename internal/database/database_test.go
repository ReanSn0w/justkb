package database

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenConfiguresSQLite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "jkb.sqlite")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for pragma, want := range map[string]string{
		"foreign_keys": "1",
		"journal_mode": "wal",
		"busy_timeout": "5000",
		"synchronous":  "1",
	} {
		var got string
		if err := db.QueryRow("PRAGMA " + pragma).Scan(&got); err != nil {
			t.Fatalf("read %s: %v", pragma, err)
		}
		if got != want {
			t.Errorf("%s=%q want %q", pragma, got, want)
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode=%o", info.Mode().Perm())
	}
}

func TestImmediateWriteTransactionsSerialize(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jkb.sqlite")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE counter(value INTEGER NOT NULL); INSERT INTO counter(value) VALUES(0)`); err != nil {
		t.Fatal(err)
	}

	first, err := BeginWrite(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Exec(`UPDATE counter SET value=value+1`); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		second, err := BeginWrite(context.Background(), db)
		if err != nil {
			done <- err
			return
		}
		defer second.Rollback()
		if _, err := second.Exec(`UPDATE counter SET value=value+1`); err != nil {
			done <- err
			return
		}
		done <- second.Commit()
	}()
	select {
	case err := <-done:
		t.Fatalf("second writer did not wait: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := first.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	var value int
	if err := db.QueryRow(`SELECT value FROM counter`).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != 2 {
		t.Fatalf("counter=%d want 2", value)
	}
}

func TestIsBusyRecognizesWriterConflict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jkb.sqlite")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE counter(value INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	first, err := BeginWrite(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Rollback()

	databaseURL := &url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	query := databaseURL.Query()
	query.Set("_txlock", "immediate")
	query.Add("_pragma", "busy_timeout(1)")
	databaseURL.RawQuery = query.Encode()
	contender, err := sql.Open("sqlite", databaseURL.String())
	if err != nil {
		t.Fatal(err)
	}
	defer contender.Close()
	_, err = contender.BeginTx(context.Background(), nil)
	if !IsBusy(err) {
		t.Fatalf("error=%v want SQLITE_BUSY", err)
	}
}

func TestDataSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jkb.sqlite")
	db, err := Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE durable(value TEXT NOT NULL); INSERT INTO durable(value) VALUES('persisted')`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var value string
	if err := db.QueryRow(`SELECT value FROM durable`).Scan(&value); err != nil {
		t.Fatal(err)
	}
	if value != "persisted" {
		t.Fatalf("value=%q", value)
	}
}
