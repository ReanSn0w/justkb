package httpapi

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/ReanSn0w/justkb/internal/frontend"
	"github.com/ReanSn0w/justkb/internal/maintenance"
	"github.com/ReanSn0w/justkb/internal/mcpserver"
)

type ReadyChecker interface {
	PingContext(context.Context) error
}

type Dependencies struct {
	Logger            *slog.Logger
	StoragePath       string
	Database          ReadyChecker
	DB                *sql.DB
	DatabaseURL       string
	SystemName        string
	CookieSecure      bool
	AttachmentMaxSize int64
}

func New(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	gate := &maintenance.Gate{}
	systemName := deps.SystemName
	if systemName == "" {
		systemName = "JKB"
	}
	mux.HandleFunc("GET /health/live", liveHandler)
	mux.HandleFunc("GET /health/ready", readyHandler(deps.StoragePath, deps.Database))
	mux.HandleFunc("GET /api/v1/system", systemHandler(systemName))
	if deps.DB != nil {
		registerAPI(mux, deps, gate)
		mux.Handle("/mcp", mcpserver.NewHandler(deps.DB))
	}
	mux.Handle("/api/", http.NotFoundHandler())
	mux.Handle("/mcp/", http.NotFoundHandler())
	mux.Handle("/", frontend.Handler())
	return middleware(deps.Logger, gate.Middleware(mux))
}
