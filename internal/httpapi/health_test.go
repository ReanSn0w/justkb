package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSystemName(t *testing.T) {
	t.Parallel()
	handler := New(Dependencies{
		Logger: slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)), StoragePath: t.TempDir(), SystemName: "GostAssistent",
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/system", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	var payload map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["name"] != "GostAssistent" {
		t.Fatalf("name = %q", payload["name"])
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
}

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
