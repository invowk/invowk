// Package cmd contains all CLI commands for invowk.
package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"invowk-cli/internal/config"
)

// Build-time variables set via ldflags
var (
	// Version is the semantic version (set via -ldflags)
	Version = "dev"
	// Commit is the git commit hash (set via -ldflags)
	Commit = "unknown"
	// BuildDate is the build timestamp (set via -ldflags)
	BuildDate = "unknown"
)

var (
	// Verbose enables verbose output
	verbose bool
	// cfgFile allows specifying a custom config file
	cfgFile string
	// Style definitions
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7C3AED"))
	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280"))
	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#10B981"))
	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#EF4444"))
	warningStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F59E0B"))
	cmdStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#3B82F6"))
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "invowk",
	Short: "A dynamically extensible command runner",
	Long: titleStyle.Render("invowk") + subtitleStyle.Render(" - A dynamically extensible command runner") + `

invowk is a powerful command runner similar to 'just' that supports
multiple execution runtimes: native shell, virtual shell (mvdan/sh),
and containerized execution (Docker/Podman).

Commands are defined in 'invowkfile' files using TOML format and can
be organized hierarchically with support for dependencies.

` + subtitleStyle.Render("Quick Start:") + `
  1. Create an invowkfile in your project directory
  2. Define commands using TOML syntax
  3. Run commands with: invowk cmd <command-name>

` + subtitleStyle.Render("Examples:") + `
  invowk cmd list           List all available commands
  invowk cmd build          Run the 'build' command
  invowk cmd test.unit      Run nested 'test.unit' command
  invowk init               Create a new invowkfile
  invowk config show        Show current configuration`,
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
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initRootConfig)

	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.config/invowk/config.toml)")

	// Add subcommands
	rootCmd.AddCommand(cmdCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(tuiCmd)
	rootCmd.AddCommand(packCmd)
}

// initRootConfig reads in config file and ENV variables if set.
func initRootConfig() {
	if cfgFile != "" {
		// Use config file from the flag
		// For now, we just load from the default location
		// TODO: Support custom config file path
	}

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		if verbose {
			fmt.Fprintln(os.Stderr, warningStyle.Render("Warning: ")+fmt.Sprintf("Failed to load config: %v", err))
		}
	}

	// Apply verbose from config if not set via flag
	if cfg != nil && !verbose {
		verbose = cfg.UI.Verbose
	}
}

// GetVerbose returns the verbose flag value
func GetVerbose() bool {
	return verbose
}
