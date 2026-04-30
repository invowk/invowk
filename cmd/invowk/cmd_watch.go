// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/watch"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"

	"github.com/spf13/cobra"
)

var (
	// errWatchDryRunConflict is returned when --ivk-watch and --ivk-dry-run are both set.
	errWatchDryRunConflict = errors.New("--ivk-watch and --ivk-dry-run cannot be used together")
	// errInvalidWatchDebounce is returned when the watch debounce duration cannot be parsed.
	errInvalidWatchDebounce = errors.New("invalid watch debounce")
)

type (
	// WatchCommandNotFoundError is returned when the specified command is not found during watch mode setup.
	WatchCommandNotFoundError struct {
		Name string
	}

	// WatchRunner owns the blocking watch loop for a configured command.
	WatchRunner interface {
		Run(context.Context) error
	}

	// WatchRunnerFactory constructs watch runners. App owns the factory so tests
	// can drive watch callbacks without starting filesystem observers.
	WatchRunnerFactory interface {
		New(watch.Config) (WatchRunner, error)
	}

	productionWatchRunnerFactory struct{}

	watchExecutionOutcome struct {
		ExitCode types.ExitCode
		Err      error
	}
)

// Error implements the error interface.
func (e *WatchCommandNotFoundError) Error() string {
	return fmt.Sprintf("command '%s' not found", e.Name)
}

func (productionWatchRunnerFactory) New(cfg watch.Config) (WatchRunner, error) {
	return watch.New(cfg)
}

func (o watchExecutionOutcome) Validate() error {
	return o.ExitCode.Validate()
}

// runWatchMode sets up file watching and re-executes the command on file changes.
// It discovers the command to get its WatchConfig, executes it once immediately,
// then starts the watcher loop. The watcher blocks until the context is cancelled
// (e.g., Ctrl+C).
func runWatchMode(cmd *cobra.Command, app *App, rootFlags *rootFlagValues, cmdFlags *cmdFlagValues, args []string) error {
	if len(args) == 0 {
		return errNoCommandSpecified
	}

	// Dry-run and watch mode are mutually exclusive: watch mode re-executes
	// on file changes, while dry-run prevents execution entirely.
	if cmdFlags.dryRun {
		return errWatchDryRunConflict
	}

	ctx := contextWithConfigPath(cmd.Context(), rootFlags.configPath)

	req := ExecuteRequest{
		Name:       args[0],
		Args:       args[1:],
		FromSource: discovery.SourceID(cmdFlags.fromSource),    //goplint:ignore -- CLI flag value, validated downstream
		ConfigPath: types.FilesystemPath(rootFlags.configPath), //goplint:ignore -- CLI flag value, may be empty
	}
	cmdInfo, resolvedReq, diags, err := app.Commands.ResolveCommand(ctx, req)
	app.Diagnostics.Render(ctx, diags, app.stderr)
	if err != nil {
		return renderAndWrapServiceError(err, req)
	}
	if cmdInfo == nil {
		return &WatchCommandNotFoundError{Name: args[0]}
	}

	args = append([]string{resolvedReq.Name}, resolvedReq.Args...)

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
				return fmt.Errorf("%w: %w", errInvalidWatchDebounce, parseErr)
			}
			debounce = d
		}
	}

	// Default to watching all files if no patterns configured.
	if len(patterns) == 0 {
		patterns = []invowkfile.GlobPattern{"**/*"}
	}

	// Build a re-execution closure that runs the command through the command
	// service and keeps command exit codes separate from infrastructure errors.
	// The closure disables watch mode on the child request to prevent recursion.
	// The changed-files parameter is unused because we re-execute the full command
	// regardless of which specific files changed.
	//
	// The caller must pass the appropriate context: the config-path-enhanced context
	// for initial execution, or the watcher's callback context for re-execution.
	// This threads cancellation signals (Ctrl+C) and context values (--ivk-config)
	// into the execution pipeline, which derives its context from cmd.Context().
	reexecute := func(execCtx context.Context, _ []string) watchExecutionOutcome {
		childFlags := *cmdFlags
		childFlags.watch = false
		cmd.SetContext(execCtx)
		req, buildErr := buildExecuteRequest(cmd, rootFlags, &childFlags, args)
		if buildErr != nil {
			return watchExecutionOutcome{Err: buildErr}
		}
		result, execErr := executeWatchRequest(cmd, app, req)
		if execErr != nil {
			return watchExecutionOutcome{Err: execErr}
		}
		return watchExecutionOutcome{ExitCode: result.ExitCode}
	}

	// Execute the command once immediately before starting the watcher.
	fmt.Fprintf(app.stdout, "%s Watch mode: initial execution of '%s'\n", VerboseHighlightStyle.Render("→"), args[0])
	if outcome := reexecute(ctx, nil); outcome.Err != nil || outcome.ExitCode != 0 {
		// Propagate context cancellation (Ctrl+C) instead of looping.
		if ctx.Err() != nil {
			return fmt.Errorf("initial execution cancelled: %w", ctx.Err())
		}
		if outcome.Err != nil {
			return fmt.Errorf("cannot start watch mode: %w", outcome.Err)
		}

		fmt.Fprintf(app.stderr, "%s Command exited with code %d\n", WarningStyle.Render("!"), outcome.ExitCode)
	}

	fmt.Fprintf(app.stdout, "\n%s Watching for changes (Ctrl+C to stop)...\n\n", VerboseHighlightStyle.Render("→"))

	// Resolve base directory: use command workdir if set, otherwise current dir.
	// Relative workdir is resolved against the invowkfile directory, matching
	// how the execution pipeline resolves it (not against os.Getwd()).
	baseDir := string(cmdInfo.Command.WorkDir)
	if baseDir != "" && !filepath.IsAbs(baseDir) {
		baseDir = filepath.Join(filepath.Dir(string(cmdInfo.FilePath)), baseDir)
	}

	// Track consecutive infrastructure failures in the OnChange
	// callback. After maxConsecutiveInfraErrors, the watcher aborts because the
	// underlying problem (deleted invowkfile, missing runtime, etc.) is unlikely
	// to be fixed by further file changes. The counter resets when the command
	// runs to completion, even if it exits non-zero.
	const maxConsecutiveInfraErrors = 3
	var consecutiveInfraErrors int

	cfg := watch.Config{
		Patterns: patterns,
		Ignore:   ignore,
		Debounce: debounce,
		BaseDir:  types.FilesystemPath(baseDir), //goplint:ignore -- from invowkfile directory resolution
		OnChange: func(cbCtx context.Context, changed []string) error {
			if clearScreen {
				fmt.Fprint(app.stdout, "\033[2J\033[H")
			}
			fmt.Fprintf(app.stdout, "%s Detected %d change(s). Re-executing '%s'...\n",
				VerboseHighlightStyle.Render("→"), len(changed), args[0])
			outcome := reexecute(cbCtx, changed)
			if outcome.Err != nil || outcome.ExitCode != 0 {
				// Propagate context cancellation (Ctrl+C) instead of looping.
				if cbCtx.Err() != nil {
					return fmt.Errorf("execution cancelled: %w", cbCtx.Err())
				}
				if outcome.Err == nil {
					consecutiveInfraErrors = 0
					fmt.Fprintf(app.stderr, "%s Command exited with code %d\n", WarningStyle.Render("!"), outcome.ExitCode)
				} else {
					consecutiveInfraErrors++
					fmt.Fprintf(app.stderr, "%s Execution failed: %v\n", WarningStyle.Render("!"), outcome.Err)
					if consecutiveInfraErrors >= maxConsecutiveInfraErrors {
						return fmt.Errorf("aborting watch: %d consecutive infrastructure failures (last: %w)", consecutiveInfraErrors, outcome.Err)
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

	w, err := app.Watchers.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to start watcher: %w", err)
	}
	// Use the config-path-enhanced context so --ivk-config is respected
	// during re-execution, not the bare cmd.Context().
	return w.Run(ctx)
}

func executeWatchRequest(cmd *cobra.Command, app *App, req ExecuteRequest) (ExecuteResult, error) {
	reqCtx := contextWithConfigPath(cmd.Context(), string(req.ConfigPath))
	cmd.SetContext(reqCtx)

	result, diags, err := app.Commands.Execute(reqCtx, req)
	app.Diagnostics.Render(reqCtx, diags, app.stderr)
	if err != nil {
		if svcErr, ok := errors.AsType[*ServiceError](err); ok {
			renderServiceError(app.stderr, svcErr)
		}
		return result, err
	}
	return result, nil
}
