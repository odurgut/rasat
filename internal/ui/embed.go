// Package ui serves the embedded web frontend from the Go binary.
package ui

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var dist embed.FS

// New returns a handler for the embedded SPA.
// Paths without a file extension fall back to index.html.
// Missing hashed assets (paths with an extension) return 404, not HTML.
func New() (http.Handler, error) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, fmt.Errorf("embed dist: %w", err)
	}
	f, err := sub.Open("index.html")
	if err != nil {
		return nil, fmt.Errorf("embed index.html: %w", err)
	}
	if err := f.Close(); err != nil {
		return nil, fmt.Errorf("close index.html: %w", err)
	}

	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name == "." {
			name = ""
		}
		if strings.HasPrefix(name, "..") {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if name == "" {
			name = "index.html"
		}

		opened, err := sub.Open(name)
		if err != nil {
			if hasExt(name) {
				http.NotFound(w, r)
				return
			}
			serveIndex(w, r, fileServer)
			return
		}
		st, statErr := opened.Stat()
		_ = opened.Close()
		if statErr != nil || st.IsDir() {
			if hasExt(name) {
				http.NotFound(w, r)
				return
			}
			serveIndex(w, r, fileServer)
			return
		}

		fileServer.ServeHTTP(w, r)
	}), nil
}

func hasExt(name string) bool {
	return strings.Contains(path.Base(name), ".")
}

func serveIndex(w http.ResponseWriter, r *http.Request, fileServer http.Handler) {
	r = r.Clone(r.Context())
	r.URL.Path = "/"
	r.URL.RawQuery = ""
	fileServer.ServeHTTP(w, r)
}
