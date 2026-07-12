package cli

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
)

type CLI struct {
	stdout           io.Writer
	stderr           io.Writer
	stdin            io.Reader
	backendNewReader func(config.Config, *http.Client) (backend.Reader, error)
	newLocalReader   func(config.Config) (backend.Reader, error)
}

func (c *CLI) confirm(prompt string) bool {
	fmt.Fprintf(c.stderr, "%s [y/N]: ", prompt)
	scanner := bufio.NewScanner(c.stdin)
	return scanner.Scan() && strings.EqualFold(strings.TrimSpace(scanner.Text()), "y")
}

var (
	version   = "0.0.11"
	commit    = "unknown"
	buildDate = "unknown"
)

func New() *CLI {
	return &CLI{
		stdout:           os.Stdout,
		stderr:           os.Stderr,
		stdin:            os.Stdin,
		backendNewReader: backend.NewReader,
		newLocalReader: func(cfg config.Config) (backend.Reader, error) {
			return backend.NewLocalReader(cfg)
		},
	}
}

func (c *CLI) Run(args []string) int {
	if translated, ok := translateStageOneArgs(args); ok {
		args = translated
	}
	return c.runCobra(args)
}
