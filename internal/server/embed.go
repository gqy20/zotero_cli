//go:build embed

package server

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:web/dist
var distFS embed.FS

func staticFileServer() http.Handler {
	sub, _ := fs.Sub(distFS, "web/dist")
	return http.FileServer(http.FS(sub))
}

// RegisterStaticRoutes serves the frontend SPA.
// API routes should be registered before calling this (they take precedence).
func RegisterStaticRoutes(mux *http.ServeMux) {
	fileServer := staticFileServer()
	mux.Handle("GET /", fileServer)
	// SPA fallback: serve index.html for all non-API routes
	mux.HandleFunc("GET /{path...}", func(w http.ResponseWriter, r *http.Request) {
		path := r.PathValue("path")
		// Don't intercept API or file requests
		if len(path) > 0 && (path[0:3] == "api" || path[0:5] == "assets") {
			fileServer.ServeHTTP(w, r)
			return
		}
		// Serve index.html for SPA routing
		index, _ := distFS.ReadFile("web/dist/index.html")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(index)
	})
}
