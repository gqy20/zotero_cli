package main

import (
	"os"

	"zotero_cli/internal/cli"
)

func main() {
	os.Exit(cli.New().Run(os.Args[1:]))
}
