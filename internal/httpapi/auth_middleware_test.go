package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ReanSn0w/justkb/internal/auth"
)

type fakeSessionAuthenticator struct {
	principal auth.Principal
}

func (f fakeSessionAuthenticator) AuthenticateSession(context.Context, string) (auth.Principal, error) {
	return f.principal, nil
}

func (f fakeSessionAuthenticator) ValidateCSRF(context.Context, string, string) bool { return true }

func TestRequireRole(t *testing.T) {
	t.Parallel()
	authenticator := fakeSessionAuthenticator{principal: auth.Principal{
		User: auth.User{ID: "user", Role: "member", IsActive: true}, SessionID: "session",
	}}
	handler := RequireSession(authenticator, RequireRole("admin", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session-token"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d", response.Code)
	}
}
