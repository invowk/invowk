// Package cmd contains all CLI commands for invowk.
package cmd

import (
	"github.com/spf13/cobra"
)

// tuiCmd is the parent command for all TUI components.
// These commands provide gum-like interactive terminal UI elements
// that can be used in shell scripts and pipelines.
var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Interactive terminal UI components",
	Long: `Interactive terminal UI components for shell scripts and pipelines.

The tui command provides a set of gum-like interactive UI elements that can
be used to build interactive shell scripts. Each subcommand provides a
different UI component:

  input    - Single-line text input
  write    - Multi-line text editor
  choose   - Select from a list of options
  confirm  - Yes/no confirmation prompt
  filter   - Fuzzy filter a list
  file     - File picker
  table    - Display and select from a table
  spin     - Show a spinner while running a command
  pager    - Scroll through content
  format   - Format text with markdown, code, or emoji
  style    - Apply styles to text

Examples:
  invowk tui input --title "What is your name?"
  invowk tui choose "Option 1" "Option 2" "Option 3"
  invowk tui confirm "Are you sure?"
  invowk tui spin --title "Working..." -- sleep 2
  echo "Hello World" | invowk tui style --foreground "#FF0000"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

func init() {
	// tuiCmd is added in root.go
}
