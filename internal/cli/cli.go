package cli

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"zotero_cli/internal/backend"
	"zotero_cli/internal/config"
)

type CLI struct {
	stdout           io.Writer
	stderr           io.Writer
	stdin            io.Reader
	currentCommand   string
	backendNewReader func(config.Config, *http.Client) (backend.Reader, error)
	newLocalReader   func(config.Config) (backend.Reader, error)
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
		return c.runCobra(translated)
	}
	return c.runLegacy(args)
}

func (c *CLI) runLegacy(args []string) int {
	if len(args) == 0 {
		c.printUsage()
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		if len(args) > 1 {
			return c.Run(append([]string{args[1]}, "--help"))
		}
		c.printUsage()
		return 0
	}

	if lookupCommand(args[0]) == nil {
		fmt.Fprintf(c.stderr, "unknown command: %s\n\n", args[0])
		c.printUsage()
		return ExitUsage
	}
	return c.dispatch(args[0], args[1:])
}

// dispatch 把 name 路由到对应的 runXxx 方法。
// commandRegistry 只管展示元数据（Name/Short/Long/Category），不分发行为，
// 以避免 commandRegistry 与 runXxx 方法形成初始化/调用环。
func (c *CLI) dispatch(name string, args []string) int {
	previousCommand := c.currentCommand
	c.currentCommand = name
	defer func() {
		c.currentCommand = previousCommand
	}()

	switch name {
	case "setup":
		return c.runSetup(args)
	case "server":
		return c.runServer(args)
	case "sync":
		return c.runSync(args)
	case "select":
		return c.runSelect(args)
	case "abstract":
		return c.runAbstract(args)
	case "relate":
		return c.runRelate(args)
	case "schema":
		return c.runSchema(args)
	case "key-info":
		return c.runKeyInfo(args)
	}
	fmt.Fprintf(c.stderr, "unknown command: %s\n\n", name)
	c.printUsage()
	return ExitUsage
}

func (c *CLI) printUsage() {
	exe := filepath.Base(os.Args[0])
	exe = strings.TrimSuffix(exe, ".exe")

	fmt.Fprintf(c.stdout, "%s is a minimal Zotero CLI.\n\n", exe)
	fmt.Fprintf(c.stdout, "Usage:\n  %s <command>\n\n", exe)

	categoryOrder := []Category{CatSetup, CatRead, CatAnnotate, CatWrite, CatDestructive}
	byCat := make(map[Category][]CommandSpec, len(categoryOrder))
	// nameWidth is the global column width for command names, derived from the
	// longest non-hidden name so descriptions align uniformly across every category.
	nameWidth := 0
	for _, s := range commandRegistry {
		if s.Hidden {
			continue
		}
		byCat[s.Category] = append(byCat[s.Category], s)
		if len(s.Name) > nameWidth {
			nameWidth = len(s.Name)
		}
	}
	for _, cat := range categoryOrder {
		specs := byCat[cat]
		if len(specs) == 0 {
			continue
		}
		prefix := "  "
		label := cat.String()
		if cat == CatDestructive {
			prefix = "  ⚠ "
			label = "Destructive (irreversible)"
		}
		fmt.Fprintf(c.stdout, "%s:\n", label)
		for _, s := range specs {
			fmt.Fprintf(c.stdout, "%s%-*s  %s\n", prefix, nameWidth, s.Name, s.Short)
		}
		fmt.Fprintln(c.stdout)
	}

	fmt.Fprint(c.stdout, `Modes (set via ZOT_MODE env, or run 'zot init'):
  web     Cloud-only via Zotero Web API; no local Zotero needed
  local   Read from local Zotero SQLite (requires ZOT_DATA_DIR)
  hybrid  Local-first with Web API fallback for unsupported features (default)
  remote  Read from a running 'zot server' over HTTP (requires ZOT_SERVER_ADDR)

Web-mode limits: local attachment/PDF commands are unavailable in web mode:
supplements, inspect-attachment, extract-text, extract-figures,
'find --fulltext/--snippet', and 'relate --aggregate'. Switch to local/hybrid
(or remote) to use them.

Environment (run 'zot config show' for full list):
  ZOT_MODE         Operating mode: web | local | hybrid | remote   (default: hybrid)
  ZOT_API_KEY      Zotero Web API key
  ZOT_LIBRARY_ID   Numeric user or group library ID
  ZOT_LIBRARY_TYPE Library type: user | group            (default: user)
  ZOT_SERVER_ADDR  URL of a running 'zot server' (remote mode)
  ZOT_DATA_DIR     Local Zotero data dir; required for local/hybrid mode
  ZOT_ALLOW_WRITE  On by default (1); set to 0 to make the library read-only.
                    Gates create/update-item, add/remove-tag, annotate,
                    create/update-collection/search.
  ZOT_ALLOW_DELETE Off by default (0) for safety; set to 1 to allow
                    delete-item/collection/search.

Run 'zot <command> -h' for usage, examples, and per-command mode support.
`)
}

func (c *CLI) printErr(err error) int {
	return c.jsonError(err, c.currentCommand)
}

func (c *CLI) jsonErrorsEnabled() bool {
	return os.Getenv("ZOT_JSON_ERRORS") == "1"
}

// confirm prompts the user on stdin and returns true only if they reply y/Y.
func (c *CLI) confirm(prompt string) bool {
	fmt.Fprintf(c.stderr, "%s [y/N]: ", prompt)
	scanner := bufio.NewScanner(c.stdin)
	if !scanner.Scan() {
		return false
	}
	return strings.ToLower(strings.TrimSpace(scanner.Text())) == "y"
}

func isHelpOnly(args []string) bool {
	if len(args) != 1 {
		return false
	}
	switch args[0] {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

func containsHelp(args []string) bool {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			return true
		}
	}
	return false
}

func (c *CLI) printCommandUsage(usage string) int {
	fmt.Fprintln(c.stdout, usage)
	return 0
}
