package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

type contextKey string

const requestIDKey contextKey = "request-id"

func middleware(logger *slog.Logger, next http.Handler) http.Handler {
	return recoverRequests(logger, logRequests(logger, withRequestID(next)))
}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" || len(id) > 128 {
			id = randomID()
		}
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		response := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(response, r)
		logger.Info("http request",
			"request_id", requestIDFromContext(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", response.status,
			"duration_ms", time.Since(started).Milliseconds(),
		)
	})
}

func recoverRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic recovered", "request_id", requestIDFromContext(r.Context()), "panic", recovered, "stack", string(debug.Stack()))
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]any{
					"code": "internal_error", "message": "Internal server error", "request_id": requestIDFromContext(r.Context()),
				}})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func requestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func randomID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "request-id-unavailable"
	}
	return hex.EncodeToString(raw[:])
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}
