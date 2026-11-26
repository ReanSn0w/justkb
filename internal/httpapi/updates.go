package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/ReanSn0w/justkb/internal/tokens"
	"github.com/ReanSn0w/justkb/internal/updates"
)

// projectReader deliberately applies only to the two polling endpoints.
func (a *api) projectReader(next http.Handler) http.Handler {
	session := a.member(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values, present := r.Header["Authorization"]
		if !present {
			session.ServeHTTP(w, r)
			return
		}
		if len(values) != 1 {
			writeAPIError(w, r, 401, "unauthenticated", "Invalid authorization", nil)
			return
		}
		parts := strings.Fields(values[0])
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			writeAPIError(w, r, 401, "unauthenticated", "Bearer token required", nil)
			return
		}
		principal, err := a.tokens.Authenticate(r.Context(), parts[1])
		if errors.Is(err, tokens.ErrInvalidToken) {
			writeAPIError(w, r, 401, "unauthenticated", "Invalid token", nil)
			return
		}
		if err != nil {
			handleError(w, r, err)
			return
		}
		if !principal.Scopes["cards:read"] {
			writeAPIError(w, r, 403, "forbidden", "cards:read scope required", nil)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey, principal)))
	})
}
func (a *api) projectUpdates(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if values, present := r.URL.Query()["limit"]; present {
		var err error
		if len(values) != 1 {
			err = updates.ErrLimit
		} else {
			limit, err = strconv.Atoi(values[0])
		}
		if err != nil || limit < 1 || limit > 500 {
			writeAPIError(w, r, 400, "invalid_limit", "limit must be between 1 and 500", nil)
			return
		}
	}
	cursors := r.URL.Query()["cursor"]
	if len(cursors) > 1 {
		writeAPIError(w, r, 400, "invalid_updates_cursor", "Supply one cursor", nil)
		return
	}
	page, err := a.updates.List(r.Context(), r.PathValue("id"), r.URL.Query().Get("cursor"), limit)
	switch {
	case errors.Is(err, updates.ErrNotFound):
		writeAPIError(w, r, 404, "project_not_found", "Project not found", nil)
	case errors.Is(err, updates.ErrCursor):
		writeAPIError(w, r, 400, "invalid_updates_cursor", "Invalid project updates cursor", nil)
	case errors.Is(err, updates.ErrReset):
		writeAPIError(w, r, 400, "updates_cursor_reset_required", "Resynchronize the project", nil)
	case errors.Is(err, updates.ErrLimit):
		writeAPIError(w, r, 400, "invalid_limit", "limit must be between 1 and 500", nil)
	case err != nil:
		handleError(w, r, err)
	default:
		writeJSON(w, http.StatusOK, page)
	}
}
