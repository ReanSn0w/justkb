package backups

import (
	"archive/tar"
	"bytes"
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
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const (
	Format           = "jkb-backup"
	FormatVersion    = 1
	MaxUploadSize    = int64(2 << 30)
	maxExtractedSize = int64(4 << 30)
)

var (
	ErrInvalidArchive = errors.New("invalid JKB backup archive")
	ErrSchemaMismatch = errors.New("backup schema version does not match current database")
	ErrUnavailable    = errors.New("PostgreSQL backup tools are unavailable")
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
	db          *sql.DB
	databaseURL string
	storageRoot string
	gate        exclusiveGate
	pgDump      string
	pgRestore   string
	now         func() time.Time
}

func NewService(db *sql.DB, databaseURL, storageRoot string, gate exclusiveGate) (*Service, error) {
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
	return &Service{
		db: db, databaseURL: databaseURL, storageRoot: root, gate: gate,
		pgDump: "pg_dump", pgRestore: "pg_restore", now: time.Now,
	}, nil
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
	dumpPath := filepath.Join(workDir, "database.dump")
	if err := s.dumpDatabase(ctx, workDir, dumpPath); err != nil {
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
	return s.gate.Exclusive(func() error {
		currentVersion, err := s.schemaVersion(ctx)
		if err != nil {
			return err
		}
		if manifest.SchemaVersion != currentVersion {
			return fmt.Errorf("%w: backup=%d current=%d", ErrSchemaMismatch, manifest.SchemaVersion, currentVersion)
		}
		rollbackDump := filepath.Join(workDir, "rollback.dump")
		if err := s.dumpDatabase(ctx, workDir, rollbackDump); err != nil {
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
		if err := s.restoreDatabase(ctx, workDir, dumpPath); err != nil {
			fileRollbackErr := rollbackStorage(s.storageRoot, previousStorage)
			databaseRollbackErr := s.restoreDatabase(ctx, workDir, rollbackDump)
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

func (s *Service) dumpDatabase(ctx context.Context, workDir, target string) error {
	dsn, environment, err := s.postgresEnvironment(workDir)
	if err != nil {
		return err
	}
	return runCommand(ctx, s.pgDump, environment,
		"--dbname", dsn,
		"--format=custom",
		"--data-only",
		"--exclude-table=public.goose_db_version",
		"--no-owner",
		"--no-privileges",
		"--file", target,
	)
}

func (s *Service) restoreDatabase(ctx context.Context, workDir, source string) error {
	if _, err := s.db.ExecContext(ctx, `
		TRUNCATE comments,card_activity,api_tokens,web_sessions,attachments,card_tags,cards,
		group_members,users,epics,tags,board_columns,user_groups RESTART IDENTITY CASCADE`); err != nil {
		return err
	}
	dsn, environment, err := s.postgresEnvironment(workDir)
	if err != nil {
		return err
	}
	return runCommand(ctx, s.pgRestore, environment,
		"--dbname", dsn,
		"--data-only",
		"--disable-triggers",
		"--no-owner",
		"--no-privileges",
		source,
	)
}

func (s *Service) postgresEnvironment(workDir string) (string, []string, error) {
	parsed, err := url.Parse(s.databaseURL)
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" || parsed.User == nil {
		return "", nil, errors.New("invalid PostgreSQL URL")
	}
	username := parsed.User.Username()
	password, hasPassword := parsed.User.Password()
	parsed.User = url.User(username)
	environment := os.Environ()
	if hasPassword {
		port := parsed.Port()
		if port == "" {
			port = "5432"
		}
		database := strings.TrimPrefix(parsed.Path, "/")
		pgpass := strings.Join([]string{
			escapePGPass(parsed.Hostname()), escapePGPass(port), escapePGPass(database),
			escapePGPass(username), escapePGPass(password),
		}, ":") + "\n"
		pgpassPath := filepath.Join(workDir, ".pgpass")
		if err := os.WriteFile(pgpassPath, []byte(pgpass), 0o600); err != nil {
			return "", nil, err
		}
		environment = append(environment, "PGPASSFILE="+pgpassPath)
	}
	return parsed.String(), environment, nil
}

func runCommand(ctx context.Context, name string, environment []string, arguments ...string) error {
	if _, err := exec.LookPath(name); err != nil {
		return ErrUnavailable
	}
	command := exec.CommandContext(ctx, name, arguments...)
	command.Env = environment
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if len(message) > 500 {
			message = message[:500]
		}
		if message != "" {
			return fmt.Errorf("%s failed: %w: %s", name, err, message)
		}
		return fmt.Errorf("%s failed: %w", name, err)
	}
	return nil
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
		err = writeFile(tarWriter, dumpPath, "database.dump")
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
		if name != "manifest.json" && name != "database.dump" && name != "storage" && !strings.HasPrefix(name, "storage/") {
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
	dumpPath := filepath.Join(workDir, "database.dump")
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

func escapePGPass(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, ":", `\:`)
}
