package backups

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"modernc.org/sqlite"
)

const (
	Format           = "jkb-backup"
	FormatVersion    = 2
	MaxUploadSize    = int64(2 << 30)
	maxExtractedSize = int64(4 << 30)
)

var (
	ErrInvalidArchive = errors.New("invalid JKB backup archive")
	ErrSchemaMismatch = errors.New("backup schema version does not match current database")
)

type exclusiveGate interface {
	Exclusive(func() error) error
}

type Manifest struct {
	Format        string    `json:"format"`
	Version       int       `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	SchemaVersion int64     `json:"schema_version"`
}

type Archive struct {
	Path    string
	Name    string
	workDir string
}

func (a Archive) Close() error { return os.RemoveAll(a.workDir) }

type Service struct {
	db           *sql.DB
	databasePath string
	storageRoot  string
	gate         exclusiveGate
	now          func() time.Time
	restore      func(context.Context, string) error
}

func NewService(db *sql.DB, databasePath, storageRoot string, gate exclusiveGate) (*Service, error) {
	databasePath, err := filepath.Abs(databasePath)
	if err != nil {
		return nil, err
	}
	root, err := filepath.Abs(storageRoot)
	if err != nil {
		return nil, err
	}
	if root == string(filepath.Separator) || filepath.Dir(root) == root {
		return nil, errors.New("backup storage root is unsafe")
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, err
	}
	service := &Service{
		db: db, databasePath: databasePath, storageRoot: root, gate: gate, now: time.Now,
	}
	service.restore = service.restoreDatabase
	return service, nil
}

func (s *Service) Create(ctx context.Context) (Archive, error) {
	var archive Archive
	err := s.gate.Exclusive(func() error {
		var err error
		archive, err = s.createLocked(ctx)
		return err
	})
	return archive, err
}

func (s *Service) createLocked(ctx context.Context) (Archive, error) {
	workDir, err := os.MkdirTemp(filepath.Dir(s.storageRoot), ".jkb-backup-*")
	if err != nil {
		return Archive{}, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(workDir)
		}
	}()
	createdAt := s.now().UTC()
	schemaVersion, err := s.schemaVersion(ctx)
	if err != nil {
		return Archive{}, err
	}
	dumpPath := filepath.Join(workDir, "database.sqlite")
	if err := s.snapshotDatabase(ctx, dumpPath); err != nil {
		return Archive{}, err
	}
	manifest := Manifest{Format: Format, Version: FormatVersion, CreatedAt: createdAt, SchemaVersion: schemaVersion}
	name := fmt.Sprintf("jkb-backup-%s.tar.gz", createdAt.Format("20060102-150405"))
	archivePath := filepath.Join(workDir, name)
	if err := writeArchive(archivePath, dumpPath, s.storageRoot, manifest); err != nil {
		return Archive{}, err
	}
	cleanup = false
	return Archive{Path: archivePath, Name: name, workDir: workDir}, nil
}

func (s *Service) Restore(ctx context.Context, source io.Reader) error {
	workDir, err := os.MkdirTemp(filepath.Dir(s.storageRoot), ".jkb-restore-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workDir)
	manifest, dumpPath, stagedStorage, err := extractArchive(source, workDir)
	if err != nil {
		return err
	}
	snapshotVersion, err := validateSQLiteSnapshot(ctx, dumpPath)
	if err != nil {
		return err
	}
	if snapshotVersion != manifest.SchemaVersion {
		return fmt.Errorf("%w: manifest=%d snapshot=%d", ErrInvalidArchive, manifest.SchemaVersion, snapshotVersion)
	}
	return s.gate.Exclusive(func() error {
		currentVersion, err := s.schemaVersion(ctx)
		if err != nil {
			return err
		}
		if manifest.SchemaVersion != currentVersion {
			return fmt.Errorf("%w: backup=%d current=%d", ErrSchemaMismatch, manifest.SchemaVersion, currentVersion)
		}
		rollbackDump := filepath.Join(workDir, "rollback.sqlite")
		if err := s.snapshotDatabase(ctx, rollbackDump); err != nil {
			return fmt.Errorf("create rollback database snapshot: %w", err)
		}
		previousStorage := filepath.Join(workDir, "previous-storage")
		if err := os.Rename(s.storageRoot, previousStorage); err != nil {
			return fmt.Errorf("stage current storage: %w", err)
		}
		if err := os.Rename(stagedStorage, s.storageRoot); err != nil {
			_ = os.Rename(previousStorage, s.storageRoot)
			return fmt.Errorf("publish restored storage: %w", err)
		}
		if err := s.restore(ctx, dumpPath); err != nil {
			fileRollbackErr := rollbackStorage(s.storageRoot, previousStorage)
			databaseRollbackErr := s.restore(ctx, rollbackDump)
			return errors.Join(fmt.Errorf("restore database: %w", err), fileRollbackErr, databaseRollbackErr)
		}
		_ = os.RemoveAll(previousStorage)
		return nil
	})
}

func (s *Service) schemaVersion(ctx context.Context) (int64, error) {
	var version int64
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(max(version_id),0) FROM goose_db_version WHERE is_applied`).Scan(&version)
	return version, err
}

func (s *Service) snapshotDatabase(ctx context.Context, target string) error {
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return withSQLiteConnection(ctx, s.db, func(connection sqliteBackupConnection) error {
		backup, err := connection.NewBackup(sqliteURI(target, "rwc"))
		if err != nil {
			return err
		}
		if _, err := backup.Step(-1); err != nil {
			_ = backup.Finish()
			return err
		}
		if err := backup.Finish(); err != nil {
			return err
		}
		_, err = validateSQLiteSnapshot(ctx, target)
		return err
	})
}

func (s *Service) restoreDatabase(ctx context.Context, source string) error {
	if _, err := validateSQLiteSnapshot(ctx, source); err != nil {
		return err
	}
	if err := withSQLiteConnection(ctx, s.db, func(connection sqliteBackupConnection) error {
		backup, err := connection.NewRestore(sqliteURI(source, "ro"))
		if err != nil {
			return err
		}
		if _, err := backup.Step(-1); err != nil {
			_ = backup.Finish()
			return err
		}
		return backup.Finish()
	}); err != nil {
		return err
	}
	return checkDatabase(ctx, s.db)
}

type sqliteBackupConnection interface {
	NewBackup(string) (*sqlite.Backup, error)
	NewRestore(string) (*sqlite.Backup, error)
}

func withSQLiteConnection(ctx context.Context, db *sql.DB, operation func(sqliteBackupConnection) error) error {
	connection, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer connection.Close()
	return connection.Raw(func(raw any) error {
		sqliteConnection, ok := raw.(sqliteBackupConnection)
		if !ok {
			return errors.New("SQLite connection does not support online backup")
		}
		return operation(sqliteConnection)
	})
}

func validateSQLiteSnapshot(ctx context.Context, source string) (int64, error) {
	db, err := sql.Open("sqlite", sqliteURI(source, "ro"))
	if err != nil {
		return 0, fmt.Errorf("%w: open SQLite snapshot: %v", ErrInvalidArchive, err)
	}
	defer db.Close()
	if err := checkDatabase(ctx, db); err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidArchive, err)
	}
	var version int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(max(version_id),0) FROM goose_db_version WHERE is_applied`).Scan(&version); err != nil {
		return 0, fmt.Errorf("%w: schema metadata: %v", ErrInvalidArchive, err)
	}
	return version, nil
}

func checkDatabase(ctx context.Context, db *sql.DB) error {
	var result string
	if err := db.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("SQLite quick_check: %s", result)
	}
	return nil
}

func sqliteURI(filePath, mode string) string {
	uri := &url.URL{Scheme: "file", Path: filepath.ToSlash(filePath)}
	query := uri.Query()
	query.Set("mode", mode)
	uri.RawQuery = query.Encode()
	return uri.String()
}

func writeArchive(target, dumpPath, storageRoot string, manifest Manifest) error {
	file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	failed := true
	defer func() {
		_ = file.Close()
		if failed {
			_ = os.Remove(target)
		}
	}()
	gzipWriter := gzip.NewWriter(file)
	tarWriter := tar.NewWriter(gzipWriter)
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err == nil {
		err = writeBytes(tarWriter, "manifest.json", manifestJSON)
	}
	if err == nil {
		err = writeFile(tarWriter, dumpPath, "database.sqlite")
	}
	if err == nil {
		err = filepath.WalkDir(storageRoot, func(current string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative, err := filepath.Rel(storageRoot, current)
			if err != nil {
				return err
			}
			name := "storage"
			if relative != "." {
				name += "/" + filepath.ToSlash(relative)
			}
			if entry.IsDir() {
				return tarWriter.WriteHeader(&tar.Header{Name: name + "/", Typeflag: tar.TypeDir, Mode: 0o750})
			}
			if entry.Type().IsRegular() {
				return writeFile(tarWriter, current, name)
			}
			return fmt.Errorf("unsupported storage entry: %s", relative)
		})
	}
	if closeErr := tarWriter.Close(); err == nil {
		err = closeErr
	}
	if closeErr := gzipWriter.Close(); err == nil {
		err = closeErr
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	failed = false
	return nil
}

func extractArchive(source io.Reader, workDir string) (Manifest, string, string, error) {
	gzipReader, err := gzip.NewReader(io.LimitReader(source, MaxUploadSize+1))
	if err != nil {
		return Manifest{}, "", "", ErrInvalidArchive
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	var total int64
	seen := map[string]bool{}
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return Manifest{}, "", "", ErrInvalidArchive
		}
		name := path.Clean(header.Name)
		if name == "." || path.IsAbs(name) || name == ".." || strings.HasPrefix(name, "../") {
			return Manifest{}, "", "", ErrInvalidArchive
		}
		if name != "manifest.json" && name != "database.sqlite" && name != "storage" && !strings.HasPrefix(name, "storage/") {
			return Manifest{}, "", "", ErrInvalidArchive
		}
		if seen[name] {
			return Manifest{}, "", "", ErrInvalidArchive
		}
		seen[name] = true
		total += header.Size
		if header.Size < 0 || total > maxExtractedSize {
			return Manifest{}, "", "", ErrInvalidArchive
		}
		target := filepath.Join(workDir, filepath.FromSlash(name))
		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, 0o750); err != nil {
				return Manifest{}, "", "", err
			}
			continue
		}
		if header.Typeflag != tar.TypeReg {
			return Manifest{}, "", "", ErrInvalidArchive
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return Manifest{}, "", "", err
		}
		file, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err != nil {
			return Manifest{}, "", "", err
		}
		_, copyErr := io.CopyN(file, tarReader, header.Size)
		closeErr := file.Close()
		if copyErr != nil || closeErr != nil {
			return Manifest{}, "", "", ErrInvalidArchive
		}
	}
	manifestBytes, err := os.ReadFile(filepath.Join(workDir, "manifest.json"))
	if err != nil {
		return Manifest{}, "", "", ErrInvalidArchive
	}
	var manifest Manifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil || manifest.Format != Format || manifest.Version != FormatVersion {
		return Manifest{}, "", "", ErrInvalidArchive
	}
	dumpPath := filepath.Join(workDir, "database.sqlite")
	storagePath := filepath.Join(workDir, "storage")
	if info, err := os.Stat(dumpPath); err != nil || !info.Mode().IsRegular() {
		return Manifest{}, "", "", ErrInvalidArchive
	}
	if info, err := os.Stat(storagePath); err != nil || !info.IsDir() {
		return Manifest{}, "", "", ErrInvalidArchive
	}
	return manifest, dumpPath, storagePath, nil
}

func writeBytes(writer *tar.Writer, name string, value []byte) error {
	if err := writer.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o600, Size: int64(len(value))}); err != nil {
		return err
	}
	_, err := writer.Write(value)
	return err
}

func writeFile(writer *tar.Writer, source, name string) error {
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if err := writer.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o600, Size: info.Size()}); err != nil {
		return err
	}
	_, err = io.Copy(writer, file)
	return err
}

func rollbackStorage(current, previous string) error {
	if err := os.RemoveAll(current); err != nil {
		return fmt.Errorf("remove failed restored storage: %w", err)
	}
	if err := os.Rename(previous, current); err != nil {
		return fmt.Errorf("restore previous storage: %w", err)
	}
	return nil
}
