// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"invowk-cli/internal/issue"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

var (
	// Version is the semantic version (set via -ldflags).
	Version = "dev"
	// Commit is the git commit hash (set via -ldflags).
	Commit = "unknown"
	// BuildDate is the build timestamp (set via -ldflags).
	BuildDate = "unknown"
)

type rootFlagValues struct {
	verbose     bool
	interactive bool
	configPath  string
}

// NewRootCommand creates the root Cobra command.
func NewRootCommand(app *App) *cobra.Command {
	rootFlags := &rootFlagValues{}

	rootCmd := &cobra.Command{
		Use:   "invowk",
		Short: "A dynamically extensible command runner",
		Long: TitleStyle.Render("invowk") + SubtitleStyle.Render(" - A dynamically extensible command runner") + `

invowk is a powerful command runner similar to 'just' that supports
multiple execution runtimes: native shell, virtual shell (mvdan/sh),
and containerized execution (Docker/Podman).

Commands are defined in 'invkfile' files using CUE format and can
be organized hierarchically with support for dependencies.

` + SubtitleStyle.Render("Quick Start:") + `
  1. Create an invkfile in your project directory
  2. Define commands using CUE syntax
  3. Run commands with: invowk cmd <command-name>

` + SubtitleStyle.Render("Examples:") + `
  invowk cmd                List all available commands
  invowk init               Create a new invkfile
  invowk config show        Show current configuration`,
	}

	rootCmd.PersistentFlags().BoolVarP(&rootFlags.verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().StringVar(&rootFlags.configPath, "config", "", "config file (default is $HOME/.config/invowk/config.cue)")
	rootCmd.PersistentFlags().BoolVarP(&rootFlags.interactive, "interactive", "i", false, "run commands in alternate screen buffer (interactive mode)")

	rootCmd.AddCommand(newCmdCommand(app, rootFlags))
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(tuiCmd)
	rootCmd.AddCommand(moduleCmd)
	rootCmd.AddCommand(internalCmd)

	return rootCmd
}

// Execute runs the invowk CLI.
func Execute() {
	app, err := NewApp(Dependencies{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize app: %v\n", err)
		os.Exit(1)
	}

	rootCmd := NewRootCommand(app)

	if err := fang.Execute(
		context.Background(),
		rootCmd,
		fang.WithVersion(getVersionString()),
		fang.WithNotifySignal(os.Interrupt),
	); err != nil {
		exitErr := (*ExitError)(nil)
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		os.Exit(1)
	}
}

// getVersionString returns a formatted version string for display.
func getVersionString() string {
	if Version == "dev" {
		return "dev (built from source)"
	}
	return fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, BuildDate)
}

// formatErrorForDisplay formats an error for user display.
func formatErrorForDisplay(err error, verboseMode bool) string {
	ae := (*issue.ActionableError)(nil)
	if errors.As(err, &ae) {
		return ae.Format(verboseMode)
	}

	return err.Error()
}
