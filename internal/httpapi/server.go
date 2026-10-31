package httpapi

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/ReanSn0w/justkb/internal/frontend"
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
	CookieSecure      bool
	AttachmentMaxSize int64
}

func New(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", liveHandler)
	mux.HandleFunc("GET /health/ready", readyHandler(deps.StoragePath, deps.Database))
	if deps.DB != nil {
		registerAPI(mux, deps)
		mux.Handle("/mcp", mcpserver.NewHandler(deps.DB))
	}
	mux.Handle("/api/", http.NotFoundHandler())
	mux.Handle("/mcp/", http.NotFoundHandler())
	mux.Handle("/", frontend.Handler())
	return middleware(deps.Logger, mux)
}
