// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/invowk/invowk/internal/watch"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"

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
	var patterns []invowkfile.GlobPattern
	var ignore []invowkfile.GlobPattern
	var debounce time.Duration
	var clearScreen bool

	if watchCfg := cmdInfo.Command.Watch; watchCfg != nil {
		patterns = watchCfg.Patterns
		ignore = watchCfg.Ignore
		clearScreen = watchCfg.ClearScreen
		if watchCfg.Debounce != "" {
			d, parseErr := watchCfg.ParseDebounce()
			if parseErr != nil {
				return fmt.Errorf("invalid watch debounce: %w", parseErr)
			}
			debounce = d
		}
	}

	// Default to watching all files if no patterns configured.
	if len(patterns) == 0 {
		patterns = []invowkfile.GlobPattern{"**/*"}
	}

	// Build a re-execution closure that runs the command through the normal pipeline.
	// The closure disables watch mode on the child request to prevent recursion.
	// The changed-files parameter is unused because we re-execute the full command
	// regardless of which specific files changed.
	//
	// The caller must pass the appropriate context: the config-path-enhanced context
	// for initial execution, or the watcher's callback context for re-execution.
	// This threads cancellation signals (Ctrl+C) and context values (--ivk-config)
	// into the execution pipeline, which derives its context from cmd.Context().
	reexecute := func(execCtx context.Context, _ []string) error {
		childFlags := *cmdFlags
		childFlags.watch = false
		cmd.SetContext(execCtx)
		return runCommand(cmd, app, rootFlags, &childFlags, args)
	}

	// Execute the command once immediately before starting the watcher.
	fmt.Fprintf(app.stdout, "%s Watch mode: initial execution of '%s'\n", VerboseHighlightStyle.Render("→"), args[0])
	if execErr := reexecute(ctx, nil); execErr != nil {
		// Propagate context cancellation (Ctrl+C) instead of looping.
		if ctx.Err() != nil {
			return fmt.Errorf("initial execution cancelled: %w", ctx.Err())
		}
		// Distinguish non-zero exit codes (command ran but reported failure)
		// from infrastructure errors (config broken, runtime missing, etc.).
		// ExitError means the command ran to completion — the user may fix their
		// code and save, so continue watching. Other errors indicate infrastructure
		// problems that watching cannot fix; abort immediately.
		if exitErr, ok := errors.AsType[*ExitError](execErr); ok {
			fmt.Fprintf(app.stderr, "%s Command exited with code %d\n", WarningStyle.Render("!"), exitErr.Code)
		} else {
			return fmt.Errorf("cannot start watch mode: %w", execErr)
		}
	}

	fmt.Fprintf(app.stdout, "\n%s Watching for changes (Ctrl+C to stop)...\n\n", VerboseHighlightStyle.Render("→"))

	// Resolve base directory: use command workdir if set, otherwise current dir.
	// Relative workdir is resolved against the invowkfile directory, matching
	// how the execution pipeline resolves it (not against os.Getwd()).
	baseDir := string(cmdInfo.Command.WorkDir)
	if baseDir != "" && !filepath.IsAbs(baseDir) {
		baseDir = filepath.Join(filepath.Dir(string(cmdInfo.FilePath)), baseDir)
	}

	// Track consecutive infrastructure (non-ExitError) failures in the OnChange
	// callback. After maxConsecutiveInfraErrors, the watcher aborts because the
	// underlying problem (deleted invowkfile, missing runtime, etc.) is unlikely
	// to be fixed by further file changes. The counter resets on success or ExitError.
	const maxConsecutiveInfraErrors = 3
	var consecutiveInfraErrors int

	cfg := watch.Config{
		Patterns:    patterns,
		Ignore:      ignore,
		Debounce:    debounce,
		ClearScreen: clearScreen,
		BaseDir:     types.FilesystemPath(baseDir),
		OnChange: func(cbCtx context.Context, changed []string) error {
			fmt.Fprintf(app.stdout, "%s Detected %d change(s). Re-executing '%s'...\n",
				VerboseHighlightStyle.Render("→"), len(changed), args[0])
			if execErr := reexecute(cbCtx, changed); execErr != nil {
				// Propagate context cancellation (Ctrl+C) instead of looping.
				if cbCtx.Err() != nil {
					return fmt.Errorf("execution cancelled: %w", cbCtx.Err())
				}
				// Distinguish non-zero exit codes from infrastructure errors.
				// ExitError means the command ran — reset the infra counter.
				// Other errors indicate infrastructure problems; escalate after
				// repeated consecutive failures.
				if exitErr, ok := errors.AsType[*ExitError](execErr); ok {
					consecutiveInfraErrors = 0
					fmt.Fprintf(app.stderr, "%s Command exited with code %d\n", WarningStyle.Render("!"), exitErr.Code)
				} else {
					consecutiveInfraErrors++
					fmt.Fprintf(app.stderr, "%s Execution failed: %v\n", WarningStyle.Render("!"), execErr)
					if consecutiveInfraErrors >= maxConsecutiveInfraErrors {
						return fmt.Errorf("aborting watch: %d consecutive infrastructure failures (last: %w)", consecutiveInfraErrors, execErr)
					}
				}
			} else {
				consecutiveInfraErrors = 0
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
	// Use the config-path-enhanced context so --ivk-config is respected
	// during re-execution, not the bare cmd.Context().
	return w.Run(ctx)
}
