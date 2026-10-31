package attachments

import (
	"bytes"
	"errors"
	"testing"
)

func TestPathRejectsTraversal(t *testing.T) {
	t.Parallel()
	service := &Service{root: t.TempDir()}
	for _, value := range []string{"../secret", "nested/file", "nested\\file", ""} {
		if _, err := service.path(value); !errors.Is(err, ErrInvalidPath) {
			t.Fatalf("path(%q) error = %v", value, err)
		}
	}
}

func TestAllowedMediaTypes(t *testing.T) {
	t.Parallel()
	if !allowedMediaType("image/png") || allowedMediaType("application/x-executable") {
		t.Fatal("media type policy mismatch")
	}
}

func TestUploadLimitBeforeDatabase(t *testing.T) {
	t.Parallel()
	service := &Service{root: t.TempDir(), maxSize: 4}
	_, err := service.Upload(t.Context(), "card", "user", "note.txt", bytes.NewBufferString("longer than four bytes"))
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("error = %v", err)
	}
}
