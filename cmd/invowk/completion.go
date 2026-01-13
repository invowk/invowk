package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// completionCmd generates shell completions
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for invowk.

To enable shell completions, run one of the following commands:

` + subtitleStyle.Render("Bash:") + `
  # Add to ~/.bashrc:
  eval "$(invowk completion bash)"

  # Or install system-wide:
  invowk completion bash > /etc/bash_completion.d/invowk

` + subtitleStyle.Render("Zsh:") + `
  # Add to ~/.zshrc:
  eval "$(invowk completion zsh)"

  # Or install to fpath:
  invowk completion zsh > "${fpath[1]}/_invowk"

` + subtitleStyle.Render("Fish:") + `
  invowk completion fish > ~/.config/fish/completions/invowk.fish

` + subtitleStyle.Render("PowerShell:") + `
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
