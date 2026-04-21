package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
)

type Server struct {
	handler *Handler
	addr    string
	srv     *http.Server
	logger  *Logger
}

func NewServer(reader backend.Reader, addr string) *Server {
	return NewServerWithLogger(reader, addr, DefaultLogger())
}

func NewServerWithLogger(reader backend.Reader, addr string, logger *Logger) *Server {
	mux := http.NewServeMux()
	h := NewHandler(reader)
	h.RegisterRoutes(mux)
	RegisterStaticRoutes(mux)

	handler := corsMiddleware(
		requestIDMiddleware(logger)(
			recoverMiddleware(logger)(
				loggingMiddleware(logger)(mux),
			),
		),
	)

	return &Server{
		handler: h,
		addr:    addr,
		logger:  logger,
		srv:     &http.Server{Addr: addr, Handler: handler},
	}
}

func (s *Server) Start() error {
	s.logger.Info("server starting", "addr", s.addr)
	return s.srv.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("server shutting down")
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

	logLevel := os.Getenv("ZOT_SERVER_LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	logger := NewLogger(os.Stdout, logLevel)
	addr := ":8080"
	s := NewServerWithLogger(reader, addr, logger)
	go func() {
		if err := s.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("server fatal error", "err", err)
			os.Exit(1)
		}
	}()
	shutdown := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}
	return shutdown, nil
}

// NewMockServerWithCustomLog creates a test server with a custom log output.
func NewMockServerWithCustomLog(logOutput io.Writer) http.Handler {
	logger := NewLogger(logOutput, "debug")
	mux := http.NewServeMux()
	h := NewHandler(&mockReader{})
	h.RegisterRoutes(mux)
	handler := requestIDMiddleware(logger)(
		recoverMiddleware(logger)(
			loggingMiddleware(logger)(corsMiddleware(mux)),
		),
	)
	return handler
}
