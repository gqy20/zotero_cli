package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"zotero_cli/internal/config"
	"zotero_cli/internal/server"
)

const usageServer = `usage: zot server [--port PORT]

Start the HTTP API server that backs remote mode (and optionally serves the
web UI when built with the 'embed' tag). Blocks until Ctrl+C (SIGINT) or
SIGTERM, then shuts down gracefully.

The server runs in the mode configured in ~/.zot/.env (web | local | hybrid).
It cannot run in remote mode — remote mode connects to an existing server.

Flags:
  --port PORT   Override the listen port (default 8021). Same as ZOT_SERVER_PORT.

Environment (see 'zot config show'):
  ZOT_MODE             Server-side data source: web | local | hybrid
  ZOT_SERVER_PORT      Listen port (default 8021)
  ZOT_SERVER_AUTH_KEY  If set, requires "Authorization: Bearer <key>"
  ZOT_SERVER_LOG_LEVEL trace|debug|info|warn|error (default info)
  ZOT_ALLOW_WRITE      0/1 — gates write endpoints (default 1)
  ZOT_ALLOW_DELETE     0/1 — gates delete endpoints (default 0)

Once running, point a remote-mode client at it:
  ZOT_MODE=remote ZOT_SERVER_ADDR=http://<host>:<port> zot find ...
`

func (c *CLI) runServer(args []string) int {
	if isHelpOnly(args) {
		return c.printCommandUsage(usageServer)
	}

	// --port N | --port=N 覆盖 ZOT_SERVER_PORT（ServeFromConfig 内部读取该 env）。
	port := ""
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case strings.HasPrefix(a, "--port="):
			port = strings.TrimPrefix(a, "--port=")
		case a == "--port":
			if i+1 >= len(args) {
				fmt.Fprintf(c.stderr, "flag --port requires a value\n\n%s", usageServer)
				return ExitUsage
			}
			port = args[i+1]
			i++
		default:
			fmt.Fprintf(c.stderr, "unknown flag: %s\n\n%s", a, usageServer)
			return ExitUsage
		}
	}
	if port != "" {
		os.Setenv("ZOT_SERVER_PORT", port)
	}

	cfg, _, err := config.Load()
	if err != nil {
		if err == config.ErrNotFound {
			fmt.Fprintln(c.stderr, "Config not found. Run 'zot init' first.")
			return ExitConfig
		}
		fmt.Fprintf(c.stderr, "failed to load config: %v\n", err)
		return ExitConfig
	}

	shutdown, err := server.ServeFromConfig(cfg)
	if err != nil {
		fmt.Fprintf(c.stderr, "failed to start server: %v\n", err)
		return ExitError
	}

	// 阻塞至收到中断信号，再优雅关闭（比旧 cmd/server 的裸 select{} 多了 graceful shutdown）。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()

	fmt.Fprintln(c.stderr, "\nshutting down...")
	shutdown()
	return ExitOK
}
