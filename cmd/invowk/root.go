// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	// noCwd disables current-directory command sources (--ivk-no-cwd flag).
	// Read it through resolveNoCwd, which also honours INVOWK_NO_CWD.
	noCwd bool
	// colorMode controls CLI ANSI color output (--ivk-color=auto|always|never).
	colorMode string //goplint:ignore -- Cobra flag binding validated via colorModeFlag.Set and parseColorMode.
}

// NewRootCommand creates the root Cobra command.
func NewRootCommand(app *App) *cobra.Command {
	rootFlags := &rootFlagValues{
		colorMode: colorModeAuto.String(),
	}

	rootCmd := &cobra.Command{
		Use:   "invowk",
		Short: "A dynamically extensible command runner",
		Long: TitleStyle.Render("invowk") + SubtitleStyle.Render(" - A dynamically extensible command runner, licensed under MPL-2.0 <https://mozilla.org/MPL/2.0/>") + `

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
	rootCmd.PersistentFlags().StringVarP(&rootFlags.configPath, "ivk-config", "c", "",
		"config file; overrides INVOWK_CONFIG and the user config file (see 'invowk config path'; "+userConfigPathsFlagHelp+")")
	rootCmd.PersistentFlags().BoolVar(&rootFlags.noCwd, noCwdFlagName, false,
		"ignore the current directory's invowkfile.cue and *.invowkmod modules (env: INVOWK_NO_CWD)")
	rootCmd.PersistentFlags().BoolVarP(&rootFlags.interactive, "ivk-interactive", "i", false, "run commands in alternate screen buffer (interactive mode)")
	rootCmd.PersistentFlags().Var(&colorModeFlag{raw: &rootFlags.colorMode, app: app}, colorFlagName,
		"control color output: auto, always, never")

	rootCmd.AddCommand(newCmdCommand(app, rootFlags))
	rootCmd.AddCommand(newInitCommand(app))
	rootCmd.AddCommand(newConfigCommand(app, rootFlags))
	rootCmd.AddCommand(newCompletionCommand())
	rootCmd.AddCommand(newTUICommand())
	rootCmd.AddCommand(newModuleCommand(app, rootFlags))
	rootCmd.AddCommand(newValidateCommand(app, rootFlags))
	rootCmd.AddCommand(newAuditCommand(app, rootFlags))
	rootCmd.AddCommand(newAgentCommand(app, rootFlags))
	rootCmd.AddCommand(newInternalCommand(app, rootFlags))

	installUsageGuards(rootCmd)

	return rootCmd
}

// Execute runs the invowk CLI. It creates the App with default dependencies,
// builds the Cobra command tree, and uses fang for graceful signal handling.
// The returned error is mapped to the process exit status by exitCodeForError.
func Execute() {
	app, err := NewApp(Dependencies{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize app: %v\n", err)
		os.Exit(1)
	}

	rootCmd := NewRootCommand(app)
	err = executeWithFang(context.Background(), app, rootCmd, os.Args[1:])
	os.Exit(int(exitCodeForError(classifyUnknownRootCommand(rootCmd, os.Args[1:], err))))
}

// executeWithFang configures color-aware fang help and error handling for rootCmd.
//
//goplint:ignore -- CLI entrypoint forwards raw process argv for pre-parse color setup.
func executeWithFang(ctx context.Context, app *App, rootCmd *cobra.Command, args []string) error {
	if argMode, hasArgMode := explicitColorModeFromArgs(args); hasArgMode && argMode.Validate() == nil {
		app.setColorMode(argMode)
	}

	// Only wrap rootCmd's stdout for fang help when color is disabled and the
	// invocation is not `invowk tui style|format` (which stay opt-in unless
	// --ivk-color=never, handled inside those commands). Leaving OutOrStdout()
	// unwrapped under `auto` preserves fang's TTY detection.
	if app.uiOut != nil && !colorEnabled(app.uiOut.Profile) && !isTUIStyleOrFormatInvocation(args) {
		rootCmd.SetOut(app.uiOut)
	}

	if app.alwaysColor && isHelpInvocation(args) {
		restore := forceFangHelpColor()
		defer restore()
	}

	sigCtx, stopSignals, tracker := notifyTerminationSignals(ctx)
	defer stopSignals()

	err := fang.Execute(
		sigCtx,
		rootCmd,
		fang.WithVersion(getVersionString(Version, Commit, BuildDate)),
		fang.WithErrorHandler(newFangErrorHandlerWithTracker(app, rootCmd, tracker)),
	)
	return mapSignalTerminationError(tracker, err)
}

func newFangErrorHandler(app *App, rootCmd *cobra.Command) fang.ErrorHandler {
	return newFangErrorHandlerWithTracker(app, rootCmd, nil)
}

func newFangErrorHandlerWithTracker(app *App, rootCmd *cobra.Command, tracker *signalTracker) fang.ErrorHandler {
	return func(w io.Writer, styles fang.Styles, err error) {
		errWriter := w
		if app != nil && app.uiErr != nil {
			errWriter = app.uiWriterForErr(rootCmd)
		}

		if isSignalHandledCancellation(tracker, err) {
			verbose, _ := rootCmd.PersistentFlags().GetBool("ivk-verbose")
			if verbose {
				if code, ok := signalExitCode(tracker.Signal()); ok {
					_, _ = fmt.Fprintf(errWriter, "invowk: command exited with status %d\n", code)
				}
			}
			return
		}

		if exitErr, ok := errors.AsType[*ExitError](err); ok && exitErr.Err == nil {
			verbose, _ := rootCmd.PersistentFlags().GetBool("ivk-verbose")
			if verbose {
				_, _ = fmt.Fprintf(errWriter, "invowk: command exited with status %d\n", exitErr.Code)
			}
			return
		}

		fang.DefaultErrorHandler(errWriter, styles, err)
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
		return fmt.Sprintf("%s (commit: %s, built: %s)\nLicense MPL-2.0: Mozilla Public License 2.0 <https://mozilla.org/MPL/2.0/>", version, commit, buildDate)
	}

	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version + "\nLicense MPL-2.0: Mozilla Public License 2.0 <https://mozilla.org/MPL/2.0/>"
	}

	return "dev (built from source)\nLicense MPL-2.0: Mozilla Public License 2.0 <https://mozilla.org/MPL/2.0/>"
}

// formatErrorForDisplay formats an error for user display.
//
//plint:render
func formatErrorForDisplay(err error, verboseMode bool) string {
	if ae, ok := errors.AsType[*issue.ActionableError](err); ok {
		return formatActionableError(ae, verboseMode).String()
	}

	return err.Error()
}
