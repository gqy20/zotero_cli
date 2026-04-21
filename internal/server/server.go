package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
)

type Server struct {
	handler *Handler
	addr    string
	srv     *http.Server
}

func NewServer(reader backend.Reader, addr string) *Server {
	mux := http.NewServeMux()
	h := NewHandler(reader)
	h.RegisterRoutes(mux)
	RegisterStaticRoutes(mux)

	handler := corsMiddleware(
		recoverMiddleware(
			loggingMiddleware(mux),
		),
	)

	return &Server{
		handler: h,
		addr:    addr,
		srv:     &http.Server{Addr: addr, Handler: handler},
	}
}

func (s *Server) Start() error {
	log.Printf("zotero-web server starting on %s", s.addr)
	return s.srv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}

func Serve(reader backend.Reader, addr string) error {
	s := NewServer(reader, addr)
	return s.Start()
}

func ServeFromConfig(cfg config.Config) (func(), error) {
	httpClient := &http.Client{Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second}
	reader, err := backend.NewReader(cfg, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}
	addr := ":8080"
	s := NewServer(reader, addr)
	go func() {
		if err := s.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()
	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}
	return shutdown, nil
}
