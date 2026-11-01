package avatars

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const MaxSize int64 = 5 << 20

var (
	ErrNotFound      = errors.New("avatar not found")
	ErrTooLarge      = errors.New("avatar is too large")
	ErrTypeForbidden = errors.New("avatar type is forbidden")
	ErrInvalidPath   = errors.New("avatar path is invalid")
)

type Download struct {
	File      *os.File
	MediaType string
}

type Service struct {
	db   *sql.DB
	root string
}

func NewService(db *sql.DB, storageRoot string) (*Service, error) {
	root, err := filepath.Abs(filepath.Join(storageRoot, "avatars"))
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, err
	}
	return &Service{db: db, root: root}, nil
}

func (s *Service) Upload(ctx context.Context, userID string, source io.Reader) (string, error) {
	finalPath, err := s.path(userID)
	if err != nil {
		return "", err
	}
	temporary, err := os.CreateTemp(s.root, ".avatar-*")
	if err != nil {
		return "", fmt.Errorf("create temporary avatar: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	limited := io.LimitReader(source, MaxSize+1)
	buffer := make([]byte, 512)
	read, readErr := io.ReadFull(limited, buffer)
	if readErr != nil && !errors.Is(readErr, io.EOF) && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		temporary.Close()
		return "", readErr
	}
	buffer = buffer[:read]
	mediaType := http.DetectContentType(buffer)
	if !allowedMediaType(mediaType) {
		temporary.Close()
		return "", ErrTypeForbidden
	}
	written, err := temporary.Write(buffer)
	size := int64(written)
	if err == nil {
		var copied int64
		copied, err = io.Copy(temporary, limited)
		size += copied
	}
	if err != nil {
		temporary.Close()
		return "", fmt.Errorf("write avatar: %w", err)
	}
	if size > MaxSize {
		temporary.Close()
		return "", ErrTooLarge
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}

	backupPath := finalPath + ".replacing"
	hadPrevious := false
	if err := os.Rename(finalPath, backupPath); err == nil {
		hadPrevious = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if err := os.Rename(temporaryPath, finalPath); err != nil {
		if hadPrevious {
			_ = os.Rename(backupPath, finalPath)
		}
		return "", fmt.Errorf("publish avatar: %w", err)
	}

	avatarURL := fmt.Sprintf("/api/v1/users/%s/avatar?v=%d", userID, time.Now().UnixNano())
	result, err := s.db.ExecContext(ctx, `UPDATE users SET avatar_url=$2,updated_at=now() WHERE id=$1 AND is_active`, userID, avatarURL)
	if err == nil {
		var affected int64
		affected, err = result.RowsAffected()
		if err == nil && affected == 0 {
			err = ErrNotFound
		}
	}
	if err != nil {
		_ = os.Remove(finalPath)
		if hadPrevious {
			_ = os.Rename(backupPath, finalPath)
		}
		return "", err
	}
	if hadPrevious {
		_ = os.Remove(backupPath)
	}
	return avatarURL, nil
}

func (s *Service) Open(ctx context.Context, userID string) (Download, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT avatar_url IS NOT NULL FROM users WHERE id=$1 AND is_active`, userID).Scan(&exists); errors.Is(err, sql.ErrNoRows) || !exists {
		return Download{}, ErrNotFound
	} else if err != nil {
		return Download{}, err
	}
	path, err := s.path(userID)
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
	buffer := make([]byte, 512)
	read, readErr := file.Read(buffer)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		file.Close()
		return Download{}, readErr
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		return Download{}, err
	}
	mediaType := http.DetectContentType(buffer[:read])
	if !allowedMediaType(mediaType) {
		file.Close()
		return Download{}, ErrTypeForbidden
	}
	return Download{File: file, MediaType: mediaType}, nil
}

func (s *Service) Delete(ctx context.Context, userID string) error {
	path, err := s.path(userID)
	if err != nil {
		return err
	}
	result, err := s.db.ExecContext(ctx, `UPDATE users SET avatar_url=NULL,updated_at=now() WHERE id=$1 AND is_active`, userID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *Service) path(userID string) (string, error) {
	if userID == "" || userID == "." || userID == ".." || filepath.Base(userID) != userID || strings.ContainsAny(userID, `/\`) {
		return "", ErrInvalidPath
	}
	return filepath.Join(s.root, userID), nil
}

func allowedMediaType(value string) bool {
	switch value {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}
