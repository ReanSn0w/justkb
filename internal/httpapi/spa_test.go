package httpapi

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMachineRoutesNeverFallBackToSPA(t *testing.T) {
	handler := New(Dependencies{Logger: slog.Default(), StoragePath: t.TempDir()})
	for _, route := range []string{"/api/unknown", "/api/v1/unknown", "/mcp/unknown"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, route, nil))
		if recorder.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d", route, recorder.Code, http.StatusNotFound)
		}
		if contentType := recorder.Header().Get("Content-Type"); contentType == "text/html" {
			t.Errorf("GET %s returned SPA HTML", route)
		}
	}
}
