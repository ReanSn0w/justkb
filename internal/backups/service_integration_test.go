package backups

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ReanSn0w/justkb/internal/board"
	"github.com/ReanSn0w/justkb/internal/maintenance"
	"github.com/ReanSn0w/justkb/internal/testdb"
)

func TestBackupRestoreIntegration(t *testing.T) {
	ctx, service, userID, storedFile, cleanup := backupFixture(t)
	defer cleanup()
	archive, err := service.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if _, err := service.db.ExecContext(ctx, `UPDATE users SET display_name='After backup' WHERE id=$1`, userID); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storedFile, []byte("after backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	restoreArchive(t, ctx, service, archive.Path)
	assertRestoredState(t, service, userID, storedFile, "Before backup", "before backup")
}

func TestRestoreRollsBackDatabaseAndFilesOnFailure(t *testing.T) {
	ctx, service, userID, storedFile, cleanup := backupFixture(t)
	defer cleanup()
	archive, err := service.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if _, err := service.db.ExecContext(ctx, `UPDATE users SET display_name='Current state' WHERE id=$1`, userID); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storedFile, []byte("current files"), 0o600); err != nil {
		t.Fatal(err)
	}
	originalRestore := service.restore
	calls := 0
	service.restore = func(ctx context.Context, source string) error {
		calls++
		if calls == 1 {
			if _, err := service.db.ExecContext(ctx, `UPDATE users SET display_name='Partially restored' WHERE id=$1`, userID); err != nil {
				return err
			}
			return errors.New("injected restore failure")
		}
		return originalRestore(ctx, source)
	}
	file, err := os.Open(archive.Path)
	if err != nil {
		t.Fatal(err)
	}
	err = service.Restore(ctx, file)
	_ = file.Close()
	if err == nil {
		t.Fatal("restore unexpectedly succeeded")
	}
	assertRestoredState(t, service, userID, storedFile, "Current state", "current files")
}

func backupFixture(t *testing.T) (context.Context, *Service, string, string, func()) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	testDatabase := testdb.Open(t)
	databasePath, db := testDatabase.Path, testDatabase.DB
	user, err := board.NewService(db).CreateUser(ctx, "backup-user", "Before backup", "", "admin", "test-password-long-enough")
	if err != nil {
		t.Fatal(err)
	}
	storage := filepath.Join(root, "files")
	if err := os.MkdirAll(storage, 0o750); err != nil {
		t.Fatal(err)
	}
	storedFile := filepath.Join(storage, "attachment")
	if err := os.WriteFile(storedFile, []byte("before backup"), 0o600); err != nil {
		t.Fatal(err)
	}
	service, err := NewService(db, databasePath, storage, &maintenance.Gate{})
	if err != nil {
		t.Fatal(err)
	}
	return ctx, service, user.ID, storedFile, func() {}
}

func restoreArchive(t *testing.T, ctx context.Context, service *Service, archivePath string) {
	t.Helper()
	file, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if err := service.Restore(ctx, file); err != nil {
		t.Fatal(err)
	}
}

func assertRestoredState(t *testing.T, service *Service, userID, storedFile, displayNameWant, fileWant string) {
	t.Helper()
	var displayName string
	if err := service.db.QueryRow(`SELECT display_name FROM users WHERE id=$1`, userID).Scan(&displayName); err != nil {
		t.Fatal(err)
	}
	if displayName != displayNameWant {
		t.Fatalf("display name=%q want %q", displayName, displayNameWant)
	}
	content, err := os.ReadFile(storedFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != fileWant {
		t.Fatalf("file=%q want %q", content, fileWant)
	}
}
