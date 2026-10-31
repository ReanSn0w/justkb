package httpapi

import "net/http"

func writeAPIError(w http.ResponseWriter, r *http.Request, status int, code, message string, details any) {
	if details == nil {
		details = map[string]any{}
	}
	writeJSON(w, status, map[string]any{"error": map[string]any{
		"code": code, "message": message, "details": details, "request_id": requestIDFromContext(r.Context()),
	}})
}
