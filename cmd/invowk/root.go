// SPDX-License-Identifier: MPL-2.0

// Package cmd contains all CLI commands for invowk.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"invowk-cli/internal/config"
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

	// Verbose enables verbose output
	verbose bool
	// cfgFile allows specifying a custom config file
	cfgFile string
	// interactive enables alternate screen buffer mode for command execution
	interactive bool

	// rootCmd represents the base command when called without any subcommands
	rootCmd = &cobra.Command{
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
  invowk cmd build          Run the 'build' command
  invowk cmd test.unit      Run nested 'test.unit' command
  invowk init               Create a new invkfile
  invowk config show        Show current configuration`,
	}
)

func init() {
	cobra.OnInitialize(initRootConfig)

	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/invowk/config.cue)")
	rootCmd.PersistentFlags().BoolVarP(&interactive, "interactive", "i", false, "run commands in alternate screen buffer (interactive mode)")

	// Add subcommands
	rootCmd.AddCommand(cmdCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(tuiCmd)
	rootCmd.AddCommand(moduleCmd)
}

// getVersionString returns a formatted version string for display.
func getVersionString() string {
	if Version == "dev" {
		return "dev (built from source)"
	}
	return fmt.Sprintf("%s (commit: %s, built: %s)", Version, Commit, BuildDate)
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Use fang.Execute for enhanced Cobra styling
	// Pass version via fang.WithVersion() since fang overrides rootCmd.Version
	if err := fang.Execute(
		context.Background(),
		rootCmd,
		fang.WithVersion(getVersionString()),
		fang.WithNotifySignal(os.Interrupt),
	); err != nil {
		var exitErr *ExitError
		if errors.As(err, &exitErr) {
			os.Exit(exitErr.Code)
		}
		os.Exit(1)
	}
}

// initRootConfig reads in config file and ENV variables if set.
func initRootConfig() {
	// Set custom config file path if provided via --config flag
	if cfgFile != "" {
		config.SetConfigFilePathOverride(cfgFile)
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		// Always surface config loading errors to the user (FR-011)
		fmt.Fprintln(os.Stderr, WarningStyle.Render("Warning: ")+formatErrorForDisplay(err, verbose))
	}

	// Apply verbose from config if not set via flag
	if cfg != nil && !verbose {
		verbose = cfg.UI.Verbose
	}

	// Apply interactive from config if not set via flag
	if cfg != nil && !interactive {
		interactive = cfg.UI.Interactive
	}
}

// formatErrorForDisplay formats an error for user display.
// If the error is an ActionableError, it uses the Format method.
// In verbose mode, shows the full error chain.
func formatErrorForDisplay(err error, verboseMode bool) string {
	var ae *issue.ActionableError
	if errors.As(err, &ae) {
		return ae.Format(verboseMode)
	}
	return err.Error()
}

// GetVerbose returns the verbose flag value
func GetVerbose() bool {
	return verbose
}

// GetInteractive returns the interactive flag value
func GetInteractive() bool {
	return interactive
}
