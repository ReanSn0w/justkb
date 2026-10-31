package httpapi

import (
	"context"
	"log/slog"
	"net/http"
)

type ReadyChecker interface {
	PingContext(context.Context) error
}

type Dependencies struct {
	Logger      *slog.Logger
	StoragePath string
	Database    ReadyChecker
}

func New(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", liveHandler)
	mux.HandleFunc("GET /health/ready", readyHandler(deps.StoragePath, deps.Database))
	return middleware(deps.Logger, mux)
}
