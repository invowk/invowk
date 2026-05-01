// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/watch"
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

	// WatchRunnerCreator constructs watch runners. App owns the creator so tests
	// can drive watch callbacks without starting filesystem observers.
	WatchRunnerCreator interface {
		Create(watch.Config) (WatchRunner, error)
	}

	productionWatchRunnerCreator struct{}
)

// Error implements the error interface.
func (e *WatchCommandNotFoundError) Error() string {
	return fmt.Sprintf("command '%s' not found", e.Name)
}

func (productionWatchRunnerCreator) Create(cfg watch.Config) (WatchRunner, error) {
	return watch.New(cfg)
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

	plan, err := commandsvc.NewWatchPlan(cmdInfo)
	if err != nil {
		return fmt.Errorf("%w: %w", errInvalidWatchDebounce, err)
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
	reexecute := func(execCtx context.Context, _ []string) commandsvc.WatchExecutionOutcome {
		childFlags := *cmdFlags
		childFlags.watch = false
		cmd.SetContext(execCtx)
		req, buildErr := buildExecuteRequest(cmd, rootFlags, &childFlags, args)
		if buildErr != nil {
			return commandsvc.WatchExecutionOutcome{Err: buildErr}
		}
		result, execErr := executeWatchRequest(cmd, app, req)
		if execErr != nil {
			return commandsvc.WatchExecutionOutcome{Err: execErr}
		}
		return commandsvc.WatchExecutionOutcome{ExitCode: result.ExitCode}
	}
	session, err := commandsvc.NewWatchSession(plan, reexecute)
	if err != nil {
		return err
	}

	// Execute the command once immediately before starting the watcher.
	fmt.Fprintf(app.stdout, "%s Watch mode: initial execution of '%s'\n", VerboseHighlightStyle.Render("→"), args[0])
	outcome, err := session.InitialExecution(ctx)
	if err != nil {
		return err
	}
	if outcome.ExitCode != 0 {
		fmt.Fprintf(app.stderr, "%s Command exited with code %d\n", WarningStyle.Render("!"), outcome.ExitCode)
	}

	fmt.Fprintf(app.stdout, "\n%s Watching for changes (Ctrl+C to stop)...\n\n", VerboseHighlightStyle.Render("→"))

	cfg := watch.Config{
		Patterns: plan.Patterns,
		Ignore:   plan.Ignore,
		Debounce: plan.Debounce,
		BaseDir:  plan.BaseDir,
		OnChange: func(cbCtx context.Context, changed []string) error {
			if plan.ClearScreen {
				fmt.Fprint(app.stdout, "\033[2J\033[H")
			}
			fmt.Fprintf(app.stdout, "%s Detected %d change(s). Re-executing '%s'...\n",
				VerboseHighlightStyle.Render("→"), len(changed), args[0])
			outcome, handleErr := session.HandleChange(cbCtx, changed)
			if outcome.Err != nil {
				fmt.Fprintf(app.stderr, "%s Execution failed: %v\n", WarningStyle.Render("!"), outcome.Err)
			} else if outcome.ExitCode != 0 {
				fmt.Fprintf(app.stderr, "%s Command exited with code %d\n", WarningStyle.Render("!"), outcome.ExitCode)
			}
			if handleErr != nil {
				return handleErr
			}
			fmt.Fprintf(app.stdout, "\n%s Watching for changes...\n\n", VerboseHighlightStyle.Render("→"))
			return nil
		},
		Stdout: app.stdout,
		Stderr: app.stderr,
	}

	w, err := app.Watchers.Create(cfg)
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
