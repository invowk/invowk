// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/invowk/invowk/internal/watch"

	"github.com/spf13/cobra"
)

// runWatchMode sets up file watching and re-executes the command on file changes.
// It discovers the command to get its WatchConfig, executes it once immediately,
// then starts the watcher loop. The watcher blocks until the context is cancelled
// (e.g., Ctrl+C).
func runWatchMode(cmd *cobra.Command, app *App, rootFlags *rootFlagValues, cmdFlags *cmdFlagValues, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command specified")
	}

	// Dry-run and watch mode are mutually exclusive: watch mode re-executes
	// on file changes, while dry-run prevents execution entirely.
	if cmdFlags.dryRun {
		return fmt.Errorf("--ivk-watch and --ivk-dry-run cannot be used together")
	}

	// Check for ambiguous commands before proceeding, consistent with normal execution.
	ctx := contextWithConfigPath(cmd.Context(), rootFlags.configPath)
	if ambErr := checkAmbiguousCommand(ctx, app, rootFlags, args); ambErr != nil {
		return ambErr
	}

	// Look up the command to get its watch configuration.
	// Use the configPath-enhanced context so --ivk-config is respected.
	result, err := app.Discovery.GetCommand(ctx, args[0])
	if err != nil {
		return err
	}
	if result.Command == nil {
		return fmt.Errorf("command '%s' not found", args[0])
	}

	cmdInfo := result.Command

	// Build the watch config from the command's schema or use defaults.
	var patterns []string
	var ignore []string
	var debounce time.Duration
	var clearScreen bool

	if cmdInfo.Command.Watch != nil {
		patterns = cmdInfo.Command.Watch.Patterns
		ignore = cmdInfo.Command.Watch.Ignore
		clearScreen = cmdInfo.Command.Watch.ClearScreen
		if cmdInfo.Command.Watch.Debounce != "" {
			d, parseErr := time.ParseDuration(cmdInfo.Command.Watch.Debounce)
			if parseErr != nil {
				return fmt.Errorf("invalid watch debounce %q: %w", cmdInfo.Command.Watch.Debounce, parseErr)
			}
			debounce = d
		}
	}

	// Default to watching all files if no patterns configured.
	if len(patterns) == 0 {
		patterns = []string{"**/*"}
	}

	// Build a re-execution closure that runs the command through the normal pipeline.
	// The closure disables watch mode on the child request to prevent recursion.
	reexecute := func(ctx context.Context, _ []string) error {
		childFlags := *cmdFlags
		childFlags.watch = false
		return runCommand(cmd, app, rootFlags, &childFlags, args)
	}

	// Execute the command once immediately before starting the watcher.
	fmt.Fprintf(app.stdout, "%s Watch mode: initial execution of '%s'\n", VerboseHighlightStyle.Render("→"), args[0])
	if execErr := reexecute(cmd.Context(), nil); execErr != nil {
		// Log but don't stop — the user may fix the error and save again.
		fmt.Fprintf(app.stderr, "%s Initial execution failed: %v\n", WarningStyle.Render("!"), execErr)
	}

	fmt.Fprintf(app.stdout, "\n%s Watching for changes (Ctrl+C to stop)...\n\n", VerboseHighlightStyle.Render("→"))

	// Resolve base directory: use command workdir if set, otherwise current dir.
	baseDir := ""
	if cmdInfo.Command.WorkDir != "" {
		baseDir = cmdInfo.Command.WorkDir
	}

	cfg := watch.Config{
		Patterns:    patterns,
		Ignore:      ignore,
		Debounce:    debounce,
		ClearScreen: clearScreen,
		BaseDir:     baseDir,
		OnChange: func(ctx context.Context, changed []string) error {
			fmt.Fprintf(app.stdout, "%s Detected %d change(s). Re-executing '%s'...\n",
				VerboseHighlightStyle.Render("→"), len(changed), args[0])
			if execErr := reexecute(ctx, changed); execErr != nil {
				fmt.Fprintf(app.stderr, "%s Execution failed: %v\n", WarningStyle.Render("!"), execErr)
			}
			fmt.Fprintf(app.stdout, "\n%s Watching for changes...\n\n", VerboseHighlightStyle.Render("→"))
			return nil
		},
		Stdout: app.stdout,
		Stderr: app.stderr,
	}

	w, err := watch.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to start watcher: %w", err)
	}
	return w.Run(cmd.Context())
}
