package frontend

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMissingDevelopmentBuildIsExplicit(t *testing.T) {
	recorder := httptest.NewRecorder()
	Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/cards/example", nil))
	_, buildErr := fs.Stat(assets, "dist/index.html")
	want := http.StatusOK
	if buildErr != nil {
		want = http.StatusServiceUnavailable
	}
	if recorder.Code != want {
		t.Fatalf("status = %d, want %d", recorder.Code, want)
	}
}
