package maintenance

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGateBlocksRegularRequestsDuringMaintenance(t *testing.T) {
	gate := &Gate{}
	handler := gate.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	started := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = gate.Exclusive(func() error {
			close(started)
			<-release
			return nil
		})
	}()
	<-started
	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/cards", nil))
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("request passed through maintenance gate")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("request remained blocked after maintenance")
	}
}

func TestGateAllowsHealthAndBackupEndpoints(t *testing.T) {
	gate := &Gate{}
	handler := gate.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	for _, path := range []string{"/health/ready", "/api/v1/admin/backups"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNoContent {
			t.Fatalf("%s status=%d", path, response.Code)
		}
	}
}
