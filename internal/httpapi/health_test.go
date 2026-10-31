package httpapi

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLive(t *testing.T) {
	t.Parallel()
	handler := New(Dependencies{Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), StoragePath: t.TempDir()})
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestReadyRequiresStorage(t *testing.T) {
	t.Parallel()
	handler := New(Dependencies{Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), StoragePath: t.TempDir() + "/missing"})
	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", response.Code)
	}
}
