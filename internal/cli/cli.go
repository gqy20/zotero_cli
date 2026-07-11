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
		if legacyWarningRequired(args, translated) {
			fmt.Fprintf(c.stderr, "warning: command %q is deprecated; use `zot %s`\n", args[0], canonicalPath(translated))
		}
		args = translated
	}
	return c.runCobra(args)
}

func canonicalPath(args []string) string {
	path := make([]string, 0, 2)
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") || len(path) == 2 {
			break
		}
		path = append(path, arg)
	}
	return strings.Join(path, " ")
}

func legacyWarningRequired(original, translated []string) bool {
	if len(original) == 0 || slicesEqual(original, translated) {
		return false
	}
	if original[0] == "help" {
		return false
	}
	for _, arg := range original {
		if arg == "-h" || arg == "--help" {
			return false
		}
	}
	switch original[0] {
	case "find", "show", "export":
		return false
	default:
		return true
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
