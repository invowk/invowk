// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// newCompletionCommand creates the `invowk completion` command.
func newCompletionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for invowk.

To enable shell completions, run one of the following commands:

` + SubtitleStyle.Render("Bash:") + `
  # Add to ~/.bashrc:
  eval "$(invowk completion bash)"

  # Or install system-wide:
  invowk completion bash > /etc/bash_completion.d/invowk

` + SubtitleStyle.Render("Zsh:") + `
  # Add to ~/.zshrc:
  eval "$(invowk completion zsh)"

  # Or install to fpath:
  invowk completion zsh > "${fpath[1]}/_invowk"

` + SubtitleStyle.Render("Fish:") + `
  invowk completion fish > ~/.config/fish/completions/invowk.fish

` + SubtitleStyle.Render("PowerShell:") + `
  invowk completion powershell | Out-String | Invoke-Expression

  # Or add to $PROFILE:
  invowk completion powershell >> $PROFILE
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}
			return nil
		},
	}
}
