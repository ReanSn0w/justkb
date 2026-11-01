package backups

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type testGate struct{}

func (testGate) Exclusive(operation func() error) error { return operation() }

func TestArchiveRoundTrip(t *testing.T) {
	root := filepath.Join(t.TempDir(), "storage")
	if err := os.MkdirAll(filepath.Join(root, "avatars"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "avatars", "user"), []byte("avatar"), 0o600); err != nil {
		t.Fatal(err)
	}
	dump := filepath.Join(t.TempDir(), "database.dump")
	if err := os.WriteFile(dump, []byte("database"), 0o600); err != nil {
		t.Fatal(err)
	}
	archive := filepath.Join(t.TempDir(), "backup.tar.gz")
	manifest := Manifest{Format: Format, Version: FormatVersion, CreatedAt: time.Now().UTC(), SchemaVersion: 3}
	if err := writeArchive(archive, dump, root, manifest); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	extractRoot := t.TempDir()
	loaded, loadedDump, loadedStorage, err := extractArchive(file, extractRoot)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SchemaVersion != manifest.SchemaVersion {
		t.Fatalf("manifest=%#v", loaded)
	}
	assertFile(t, loadedDump, "database")
	assertFile(t, filepath.Join(loadedStorage, "avatars", "user"), "avatar")
}

func TestExtractRejectsTraversal(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "malicious.tar.gz")
	file, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	content := []byte("bad")
	if err := tarWriter.WriteHeader(&tar.Header{Name: "../outside", Mode: 0o600, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	_, _ = tarWriter.Write(content)
	_ = tarWriter.Close()
	_ = gzipWriter.Close()
	_ = file.Close()
	file, _ = os.Open(archive)
	defer file.Close()
	if _, _, _, err := extractArchive(file, t.TempDir()); err == nil {
		t.Fatal("expected traversal archive to be rejected")
	}
}

func TestPostgresPasswordIsKeptOutOfCommandURL(t *testing.T) {
	service := &Service{databaseURL: "postgres://jkb:s3cret@db:5432/jkb?sslmode=disable"}
	workDir := t.TempDir()
	dsn, environment, err := service.postgresEnvironment(workDir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dsn, "s3cret") {
		t.Fatal("password leaked into command URL")
	}
	if !strings.Contains(strings.Join(environment, "\n"), "PGPASSFILE=") {
		t.Fatal("PGPASSFILE was not configured")
	}
}

func TestUnavailablePostgresTool(t *testing.T) {
	err := runCommand(context.Background(), filepath.Join(t.TempDir(), "missing"), os.Environ())
	if err == nil || !strings.Contains(err.Error(), ErrUnavailable.Error()) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertFile(t *testing.T, path, expected string) {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != expected {
		t.Fatalf("%s=%q", path, content)
	}
}
