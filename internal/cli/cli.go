package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	stdout = io.Writer(os.Stdout)
	stderr = io.Writer(os.Stderr)

	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func Run(args []string) int {
	if len(args) == 0 {
		printUsage()
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		printUsage()
		return 0
	case "version":
		printVersion()
		return 0
	case "config":
		return runConfig(args[1:])
	case "find":
		return runFind(args[1:])
	case "show":
		return runShow(args[1:])
	case "cite":
		return runCite(args[1:])
	case "export":
		return runExport(args[1:])
	case "collections":
		return runCollections(args[1:])
	case "notes":
		return runNotes(args[1:])
	case "tags":
		return runTags(args[1:])
	case "searches":
		return runSearches(args[1:])
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		printUsage()
		return 2
	}
}

func printUsage() {
	exe := filepath.Base(os.Args[0])
	fmt.Fprintf(stdout, `%s is a minimal Zotero CLI.

Usage:
  %s <command>

Commands:
  version        Show CLI version
  config path    Print config path
  config init    Create a starter config file
  config show    Show active config with masked secrets
  find           Search items in the configured Zotero library
  show           Show item details
  cite           Generate a citation or bibliography entry
  export         Export bibliography entries
  collections    List collections
  notes          List notes
  tags           List tags
  searches       List saved searches
`, exe, exe)
}

func printVersion() {
	fmt.Fprintf(stdout, "zot %s\n", version)
	fmt.Fprintf(stdout, "commit: %s\n", commit)
	fmt.Fprintf(stdout, "built: %s\n", buildDate)
}

func printConfigUsage() {
	fmt.Fprint(stdout, `Usage:
  zot config path
  zot config init
  zot config init --example
  zot config show
`)
}

func printErr(err error) int {
	fmt.Fprintln(stderr, "error:", err)
	return 1
}
