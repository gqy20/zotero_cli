package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
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

func NewServerWithDir(reader backend.Reader, addr string, dataDir string) *Server {
	return NewServerWithDirAndLogger(reader, addr, dataDir, DefaultLogger())
}

func NewServerWithLogger(reader backend.Reader, addr string, logger *Logger) *Server {
	return NewServerWithDirAndLogger(reader, addr, "", logger)
}

func NewServerWithDirAndLogger(reader backend.Reader, addr string, dataDir string, logger *Logger) *Server {
	return NewServerWithPermissionsAndLogger(reader, addr, dataDir, false, false, logger)
}

func NewServerWithPermissionsAndLogger(reader backend.Reader, addr string, dataDir string, allowWrite bool, allowDelete bool, logger *Logger) *Server {
	mux := http.NewServeMux()
	h := NewHandlerWithPermissions(reader, dataDir, allowWrite, allowDelete)
	h.RegisterRoutes(mux)
	RegisterStaticRoutes(mux)

	authKey := os.Getenv("ZOT_SERVER_AUTH_KEY")

	handler := corsMiddleware(
		authMiddleware(authKey)(
			requestIDMiddleware(logger)(
				recoverMiddleware(logger)(
					loggingMiddleware(logger)(mux),
				),
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
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}
	localURL, networkURLs := serverURLs(listener.Addr())
	serverAddr := localURL
	if len(networkURLs) > 0 {
		serverAddr = networkURLs[0]
	}
	s.logger.Info(
		"server ready",
		"listen_addr", listener.Addr().String(),
		"server_addr", serverAddr,
		"local_url", localURL,
		"network_urls", networkURLs,
		"remote_config", "zot config init --mode remote --server-addr "+serverAddr,
	)
	return s.srv.Serve(listener)
}

func serverURLs(addr net.Addr) (string, []string) {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "", nil
	}
	if ip := net.ParseIP(host); ip != nil && !ip.IsUnspecified() {
		url := httpURL(ip.String(), port)
		if ip.IsLoopback() {
			return url, nil
		}
		return "", []string{url}
	}

	localURL := httpURL("localhost", port)
	seen := make(map[string]struct{})
	var networkURLs []string
	interfaces, err := net.Interfaces()
	if err != nil {
		return localURL, nil
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, candidate := range addrs {
			ip, _, err := net.ParseCIDR(candidate.String())
			if err != nil || !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}
			url := httpURL(ip.String(), port)
			if _, ok := seen[url]; ok {
				continue
			}
			seen[url] = struct{}{}
			networkURLs = append(networkURLs, url)
		}
	}
	sort.Slice(networkURLs, func(i, j int) bool {
		iIPv6 := isIPv6URL(networkURLs[i])
		jIPv6 := isIPv6URL(networkURLs[j])
		if iIPv6 != jIPv6 {
			return !iIPv6
		}
		return networkURLs[i] < networkURLs[j]
	})
	return localURL, networkURLs
}

func httpURL(host, port string) string {
	return "http://" + net.JoinHostPort(host, port)
}

func isIPv6URL(rawURL string) bool {
	return len(rawURL) > len("http://") && rawURL[len("http://")] == '['
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
	if cfg.Mode == "remote" {
		return nil, fmt.Errorf("cannot start server in remote mode; remote mode connects to an existing server, not creates one")
	}

	httpClient := &http.Client{Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second}
	reader, err := backend.NewReader(cfg, httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create reader: %w", err)
	}
	dataDir := cfg.DataDir
	if dataDir == "" {
		switch typed := reader.(type) {
		case *backend.LocalReader:
			dataDir = typed.DataDir
		case *backend.HybridReader:
			if local := typed.LocalReader(); local != nil {
				dataDir = local.DataDir
			}
		}
	}

	logLevel := os.Getenv("ZOT_SERVER_LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	logger := NewLogger(os.Stdout, logLevel)
	addr := ":8021"
	if port := os.Getenv("ZOT_SERVER_PORT"); port != "" {
		addr = ":" + port
	}
	s := NewServerWithPermissionsAndLogger(reader, addr, dataDir, cfg.AllowWrite, cfg.AllowDelete, logger)
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
