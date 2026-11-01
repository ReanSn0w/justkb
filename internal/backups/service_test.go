package backups

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
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
	dump := filepath.Join(t.TempDir(), "database.sqlite")
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

func TestExtractRejectsLegacyBackupFormat(t *testing.T) {
	root := filepath.Join(t.TempDir(), "storage")
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}
	databasePath := filepath.Join(t.TempDir(), "database.sqlite")
	if err := os.WriteFile(databasePath, []byte("legacy"), 0o600); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(t.TempDir(), "legacy.tar.gz")
	manifest := Manifest{Format: Format, Version: 1, CreatedAt: time.Now().UTC(), SchemaVersion: 4}
	if err := writeArchive(archivePath, databasePath, root, manifest); err != nil {
		t.Fatal(err)
	}
	archive, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if _, _, _, err := extractArchive(archive, t.TempDir()); !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("error=%v want ErrInvalidArchive", err)
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
