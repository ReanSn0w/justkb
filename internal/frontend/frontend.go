package frontend

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// assets contains the production build produced by Vite.
//
//go:embed all:dist
var assets embed.FS

// Handler serves immutable frontend assets and falls back to index.html for
// client-side routes. API and MCP route exclusions are enforced by the parent
// HTTP mux, so an unknown machine endpoint can never receive HTML.
func Handler() http.Handler {
	dist, err := fs.Sub(assets, "dist")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name != "." && name != "" {
			if info, statErr := fs.Stat(dist, name); statErr == nil && !info.IsDir() {
				files.ServeHTTP(w, r)
				return
			}
		}
		if _, statErr := fs.Stat(dist, "index.html"); statErr != nil {
			http.Error(w, "frontend build is not available", http.StatusServiceUnavailable)
			return
		}
		request := r.Clone(r.Context())
		request.URL.Path = "/"
		files.ServeHTTP(w, request)
	})
}
