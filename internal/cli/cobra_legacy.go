package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var legacyOnlyCommands = map[string]string{
	"setup":    "use `zot config init` (and `zot config init --pdf` for PDF setup)",
	"select":   "open the item from Zotero Desktop; this platform-specific bridge is not part of CLI v2",
	"abstract": "use `zot item show KEY --json` and read the abstract field",
	"relate":   "this experimental relation-graph command has no stable CLI v2 replacement",
	"key-info": "use `zot config check` to validate the configured API key and library access",
}

func (c *CLI) addLegacyOnlyCommands(root *cobra.Command) {
	for name, replacement := range legacyOnlyCommands {
		name, replacement := name, replacement
		root.AddCommand(&cobra.Command{
			Use:                name,
			Hidden:             true,
			DisableFlagParsing: true,
			Args:               cobra.ArbitraryArgs,
			RunE: func(*cobra.Command, []string) error {
				return &exitError{code: ExitUsage, err: fmt.Errorf("legacy command %q is no longer executed: %s", name, replacement)}
			},
		})
	}
}
