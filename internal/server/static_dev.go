//go:build !embed

package server

import (
	"net/http"
)

// RegisterStaticRoutes in dev mode redirects to the Vite dev server.
// In production (with 'embed' build tag), use embed.go instead.
func RegisterStaticRoutes(mux *http.ServeMux) {
	// No-op in dev mode: frontend is served by Vite on :5173
	// API routes are still registered and proxied by Vite
}
