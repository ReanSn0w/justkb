package httpapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/ReanSn0w/justkb/internal/auth"
)

const sessionCookieName = "jkb_session"

type principalKeyType string

const principalKey principalKeyType = "principal"

type SessionAuthenticator interface {
	AuthenticateSession(context.Context, string) (auth.Principal, error)
	ValidateCSRF(context.Context, string, string) bool
}

func RequireSession(authenticator SessionAuthenticator, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" {
			writeAPIError(w, r, http.StatusUnauthorized, "unauthenticated", "Authentication required", nil)
			return
		}
		principal, err := authenticator.AuthenticateSession(r.Context(), cookie.Value)
		if errors.Is(err, auth.ErrUnauthenticated) {
			writeAPIError(w, r, http.StatusUnauthorized, "unauthenticated", "Authentication required", nil)
			return
		}
		if err != nil {
			writeAPIError(w, r, http.StatusInternalServerError, "internal_error", "Internal server error", nil)
			return
		}
		if requiresCSRF(r.Method) && !authenticator.ValidateCSRF(r.Context(), principal.SessionID, r.Header.Get("X-CSRF-Token")) {
			writeAPIError(w, r, http.StatusForbidden, "invalid_csrf", "CSRF token is invalid", nil)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey, principal)))
	})
}

func RequireRole(role string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := PrincipalFromContext(r.Context())
		if !ok || principal.User.Role != role {
			writeAPIError(w, r, http.StatusForbidden, "forbidden", "Insufficient permissions", nil)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func PrincipalFromContext(ctx context.Context) (auth.Principal, bool) {
	principal, ok := ctx.Value(principalKey).(auth.Principal)
	return principal, ok
}

func requiresCSRF(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}
