// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime/debug"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"

	"github.com/invowk/invowk/internal/issue"
)

var (
	// Version is the semantic version (set via -ldflags).
	Version = "dev"
	// Commit is the git commit hash (set via -ldflags).
	Commit = "unknown"
	// BuildDate is the build timestamp (set via -ldflags).
	BuildDate = "unknown"
)

// rootFlagValues holds the persistent flag bindings for the root command.
// These flags are inherited by all subcommands via Cobra's persistent flag mechanism.
type rootFlagValues struct {
	// verbose enables verbose output across all subcommands.
	verbose bool
	// interactive enables alternate screen buffer (interactive mode) for execution.
	interactive bool
	// configPath overrides the default config file location (--ivk-config flag).
	configPath string
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

Commands are defined in 'invowkfile' files using CUE format and can
be organized hierarchically with support for dependencies.

` + SubtitleStyle.Render("Quick Start:") + `
  1. Create an invowkfile in your project directory
  2. Define commands using CUE syntax
  3. Run commands with: invowk cmd <command-name>

` + SubtitleStyle.Render("Examples:") + `
  invowk cmd                List all available commands
  invowk init               Create a new invowkfile
  invowk config show        Show current configuration`,
	}

	rootCmd.PersistentFlags().BoolVarP(&rootFlags.verbose, "ivk-verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().StringVarP(&rootFlags.configPath, "ivk-config", "c", "", "config file (default is $HOME/.config/invowk/config.cue)")
	rootCmd.PersistentFlags().BoolVarP(&rootFlags.interactive, "ivk-interactive", "i", false, "run commands in alternate screen buffer (interactive mode)")

	rootCmd.AddCommand(newCmdCommand(app, rootFlags))
	rootCmd.AddCommand(newInitCommand())
	rootCmd.AddCommand(newConfigCommand(app))
	rootCmd.AddCommand(newCompletionCommand())
	rootCmd.AddCommand(newTUICommand())
	rootCmd.AddCommand(newModuleCommand(app))
	rootCmd.AddCommand(newValidateCommand(app))
	rootCmd.AddCommand(newInternalCommand())

	return rootCmd
}

// Execute runs the invowk CLI. It creates the App with default dependencies,
// builds the Cobra command tree, and uses fang for graceful signal handling.
// Non-zero exit codes from ExitError are propagated to os.Exit.
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
		fang.WithVersion(getVersionString(Version, Commit, BuildDate)),
		fang.WithNotifySignal(os.Interrupt),
	); err != nil {
		if exitErr, ok := errors.AsType[*ExitError](err); ok {
			os.Exit(int(exitErr.Code))
		}
		os.Exit(1)
	}
}

// getVersionString returns a formatted version string for display.
// Precedence: ldflags version > debug.ReadBuildInfo() module version > "dev (built from source)".
// This ensures go-install binaries show their module version (e.g., "v1.0.0")
// instead of the default "dev" when ldflags are not set.
//
//plint:render
func getVersionString(version, commit, buildDate string) string {
	if version != "dev" {
		return fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, buildDate)
	}

	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	return "dev (built from source)"
}

// formatErrorForDisplay formats an error for user display.
//
//plint:render
func formatErrorForDisplay(err error, verboseMode bool) string {
	if ae, ok := errors.AsType[*issue.ActionableError](err); ok {
		return ae.Format(verboseMode)
	}

	return err.Error()
}
