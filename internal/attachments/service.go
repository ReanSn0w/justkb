package attachments

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ReanSn0w/justkb/internal/archive"
	appdb "github.com/ReanSn0w/justkb/internal/database"
	"github.com/ReanSn0w/justkb/internal/ids"
)

var (
	ErrNotFound      = errors.New("attachment not found")
	ErrTooLarge      = errors.New("attachment is too large")
	ErrTypeForbidden = errors.New("attachment type is forbidden")
	ErrInvalidPath   = errors.New("attachment path is invalid")
)

type Metadata struct {
	ID           string    `json:"id"`
	CardID       *string   `json:"card_id,omitempty"`
	EpicID       *string   `json:"epic_id,omitempty"`
	OriginalName string    `json:"original_name"`
	MediaType    string    `json:"media_type"`
	Size         int64     `json:"size"`
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	storageName  string
}

type Download struct {
	Metadata Metadata
	File     *os.File
}

type Service struct {
	db      *sql.DB
	root    string
	maxSize int64
}

func NewService(db *sql.DB, root string, maxSize int64) (*Service, error) {
	if maxSize <= 0 {
		return nil, errors.New("attachment max size must be positive")
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absolute, 0o750); err != nil {
		return nil, err
	}
	return &Service{db: db, root: absolute, maxSize: maxSize}, nil
}

func (s *Service) Upload(ctx context.Context, cardID, actorUserID, originalName string, source io.Reader) (Metadata, error) {
	return s.upload(ctx, cardID, "", actorUserID, originalName, source)
}

func (s *Service) UploadEpic(ctx context.Context, epicID, actorUserID, originalName string, source io.Reader) (Metadata, error) {
	return s.upload(ctx, "", epicID, actorUserID, originalName, source)
}

func (s *Service) upload(ctx context.Context, cardID, epicID, actorUserID, originalName string, source io.Reader) (Metadata, error) {
	if (cardID == "") == (epicID == "") || actorUserID == "" || strings.TrimSpace(originalName) == "" {
		return Metadata{}, errors.New("one owner, actor and filename are required")
	}
	temporary, err := os.CreateTemp(s.root, ".upload-*")
	if err != nil {
		return Metadata{}, fmt.Errorf("create temporary attachment: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	limited := io.LimitReader(source, s.maxSize+1)
	buffer := make([]byte, 512)
	read, readErr := io.ReadFull(limited, buffer)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		temporary.Close()
		return Metadata{}, readErr
	}
	buffer = buffer[:read]
	mediaType := http.DetectContentType(buffer)
	if !allowedMediaType(mediaType) {
		temporary.Close()
		return Metadata{}, ErrTypeForbidden
	}
	initialWritten, err := temporary.Write(buffer)
	written := int64(initialWritten)
	if err == nil {
		var copied int64
		copied, err = io.Copy(temporary, limited)
		written += copied
	}
	if err != nil {
		temporary.Close()
		return Metadata{}, fmt.Errorf("write attachment: %w", err)
	}
	if written > s.maxSize {
		temporary.Close()
		return Metadata{}, ErrTooLarge
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return Metadata{}, err
	}
	if err := temporary.Close(); err != nil {
		return Metadata{}, err
	}

	storageName, err := randomName()
	if err != nil {
		return Metadata{}, err
	}
	finalPath, err := s.path(storageName)
	if err != nil {
		return Metadata{}, err
	}
	tx, err := appdb.BeginWrite(ctx, s.db)
	if err != nil {
		return Metadata{}, err
	}
	defer tx.Rollback()
	if err := archive.EnsureEditable(ctx, tx, cardID, epicID); err != nil {
		return Metadata{}, err
	}
	var metadata Metadata
	err = tx.QueryRowContext(ctx, `
		INSERT INTO attachments(id,card_id,epic_id,original_name,storage_name,media_type,size,created_by)
		VALUES($1,NULLIF($2,''),NULLIF($3,''),$4,$5,$6,$7,$8)
		RETURNING id,card_id,epic_id,original_name,media_type,size,created_by,created_at`,
		ids.New(), cardID, epicID, filepath.Base(originalName), storageName, mediaType, written, actorUserID,
	).Scan(&metadata.ID, &metadata.CardID, &metadata.EpicID, &metadata.OriginalName, &metadata.MediaType, &metadata.Size, &metadata.CreatedBy, &metadata.CreatedAt)
	if err != nil {
		return Metadata{}, err
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		return Metadata{}, fmt.Errorf("publish attachment: %w", err)
	}
	if err := tx.Commit(); err != nil {
		_ = os.Remove(finalPath)
		return Metadata{}, err
	}
	metadata.storageName = storageName
	return metadata, nil
}

func (s *Service) ListEpic(ctx context.Context, epicID string) ([]Metadata, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,card_id,epic_id,original_name,media_type,size,created_by,created_at
		FROM attachments WHERE epic_id=$1 ORDER BY created_at,id`, epicID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Metadata, 0)
	for rows.Next() {
		var item Metadata
		if err := rows.Scan(&item.ID, &item.CardID, &item.EpicID, &item.OriginalName, &item.MediaType, &item.Size, &item.CreatedBy, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) Open(ctx context.Context, id string) (Download, error) {
	metadata, err := s.metadata(ctx, id)
	if err != nil {
		return Download{}, err
	}
	path, err := s.path(metadata.storageName)
	if err != nil {
		return Download{}, err
	}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return Download{}, ErrNotFound
	}
	if err != nil {
		return Download{}, err
	}
	return Download{Metadata: metadata, File: file}, nil
}

func (s *Service) Delete(ctx context.Context, id string) error {
	metadata, err := s.metadata(ctx, id)
	if err != nil {
		return err
	}
	path, err := s.path(metadata.storageName)
	if err != nil {
		return err
	}
	tombstone := path + ".deleting"
	if err := os.Rename(path, tombstone); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ErrNotFound
		}
		return err
	}
	restore := true
	defer func() {
		if restore {
			_ = os.Rename(tombstone, path)
		}
	}()
	tx, err := appdb.BeginWrite(ctx, s.db)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var cardID, epicID string
	if metadata.CardID != nil {
		cardID = *metadata.CardID
	}
	if metadata.EpicID != nil {
		epicID = *metadata.EpicID
	}
	if err := archive.EnsureEditable(ctx, tx, cardID, epicID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM attachments WHERE id=$1`, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	restore = false
	if err := os.Remove(tombstone); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove attachment tombstone: %w", err)
	}
	return nil
}

func (s *Service) metadata(ctx context.Context, id string) (Metadata, error) {
	var metadata Metadata
	err := s.db.QueryRowContext(ctx, `SELECT id,card_id,epic_id,original_name,storage_name,media_type,size,created_by,created_at FROM attachments WHERE id=$1`, id).Scan(
		&metadata.ID, &metadata.CardID, &metadata.EpicID, &metadata.OriginalName, &metadata.storageName, &metadata.MediaType, &metadata.Size, &metadata.CreatedBy, &metadata.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Metadata{}, ErrNotFound
	}
	return metadata, err
}

func (s *Service) path(storageName string) (string, error) {
	if storageName == "" || filepath.Base(storageName) != storageName || strings.ContainsAny(storageName, `/\`) {
		return "", ErrInvalidPath
	}
	path := filepath.Join(s.root, storageName)
	relative, err := filepath.Rel(s.root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", ErrInvalidPath
	}
	return path, nil
}

func randomName() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func allowedMediaType(value string) bool {
	base, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false
	}
	switch base {
	case "text/plain", "image/jpeg", "image/png", "image/gif", "image/webp", "application/pdf", "application/zip":
		return true
	default:
		return false
	}
}
