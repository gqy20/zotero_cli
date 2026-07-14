package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"zotero_cli/internal/app"
)

func (c *CLI) newCompletionCommand(opts *globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:                   "completion <bash|zsh|fish|powershell>",
		Short:                 "Generate shell completion",
		Args:                  cobra.ExactArgs(1),
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonOutputRequested(opts) {
				return &exitError{code: ExitUsage, err: app.NewUsageError("completion output is a shell script and is only available in text mode")}
			}
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				return &exitError{code: ExitUsage, err: fmt.Errorf("unsupported shell %q", args[0])}
			}
		},
	}
}
