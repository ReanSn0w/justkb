package frontend

import (
	"bytes"
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
func Handler(options ...Branding) http.Handler {
	branding := Branding{}
	if len(options) > 0 {
		branding = options[0]
	}
	filesByPath, head := brandingAssets(branding)
	dist, err := fs.Sub(assets, "dist")
	if err != nil {
		panic(err)
	}
	files := http.FileServer(http.FS(dist))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if serveBranding(w, r, filesByPath) {
			return
		}
		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name != "." && name != "" {
			if info, statErr := fs.Stat(dist, name); statErr == nil && !info.IsDir() && name != "index.html" {
				files.ServeHTTP(w, r)
				return
			}
		}
		if _, statErr := fs.Stat(dist, "index.html"); statErr != nil {
			http.Error(w, "frontend build is not available", http.StatusServiceUnavailable)
			return
		}
		index, _ := fs.ReadFile(dist, "index.html")
		index = bytes.Replace(index, []byte("<title>JKB</title>"), []byte(head), 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, "index.html", timeZero, bytes.NewReader(index))

	})
}
