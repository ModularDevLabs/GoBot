package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed webui/dist/*
var embeddedFS embed.FS

func (s *Server) handleStatic() http.Handler {
	sub, err := fs.Sub(embeddedFS, "webui/dist")
	if err != nil {
		return http.NotFoundHandler()
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/login" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// Prevent stale dashboard assets after local rebuilds.
		w.Header().Set("Cache-Control", "no-store")

		if r.URL.Path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Try to serve the asset; fallback to index.html for SPA routes.
		_, err := fs.Stat(sub, strings.TrimPrefix(path.Clean(r.URL.Path), "/"))
		if err != nil {
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
}
