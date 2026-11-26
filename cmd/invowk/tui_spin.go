// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiclient"
	"github.com/invowk/invowk/internal/tuiwire"
	"github.com/invowk/invowk/pkg/types"

	"github.com/spf13/cobra"
)

const (
	// tuiSpinCommandWaitDelay bounds Wait after the wrapped command was killed,
	// so a grandchild that holds the output pipe cannot keep spin alive. It
	// must stay below tui.SpinCancelWait, so that a cancelled command normally
	// ends before SpinWithAction stops waiting for it.
	tuiSpinCommandWaitDelay = 10 * time.Second

	// The build fails unless tuiSpinCommandWaitDelay is below
	// tui.SpinCancelWait: a negative constant does not convert to uint64.
	_ = uint64(tui.SpinCancelWait - tuiSpinCommandWaitDelay - 1)

	// tuiSpinNotRunErrMsg is the message of errTUISpinNotRun.
	tuiSpinNotRunErrMsg = "spin: the command did not run"
)

// errTUISpinNotRun reports a spin function that returned without running the
// command and without an error; tui.SpinWithAction never does that.
var errTUISpinNotRun = errors.New(tuiSpinNotRunErrMsg)

type (
	tuiSpinRunner interface {
		Run(context.Context, string, []string) tuiSpinRunResult
	}

	tuiSpinAction func(context.Context, tui.SpinOptions, func(context.Context)) error

	tuiSpinRunResult struct {
		//goplint:ignore -- CLI adapter carries raw process output from exec.CombinedOutput.
		Output   []byte
		ExitCode types.ExitCode
		Err      error
	}

	execTUISpinRunner struct{}
)

// newTUISpinCommand creates the `invowk tui spin` command.
func newTUISpinCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spin [flags] -- command [args...]",
		Short: "Show a spinner while running a command",
		Long: `Display a spinner animation while running a command.

The command's output is captured and displayed after the spinner completes.
Use "--" to separate spin flags from the command to execute.

Available spinner types: line, dot, minidot, jump, pulse, points,
globe, moon, monkey, meter, hamburger, ellipsis

Examples:
  # Run a command with spinner
  invowk tui spin --title "Installing..." -- npm install

  # Different spinner type
  invowk tui spin --type globe --title "Downloading..." -- curl -O https://example.com/file

  # Sleep example
  invowk tui spin --title "Please wait..." -- sleep 3

  # Use in shell script
  invowk tui spin --title "Building project..." -- make build`,
		RunE: runTuiSpin,
	}

	cmd.Flags().String("title", "Loading...", "text displayed next to the spinner")
	cmd.Flags().String("type", "line", "spinner animation type")

	return cmd
}

func runTuiSpin(cmd *cobra.Command, args []string) error {
	return runTuiSpinWithRunner(cmd, args, execTUISpinRunner{}, tui.SpinWithAction, cmd.OutOrStdout())
}

func runTuiSpinWithRunner(cmd *cobra.Command, args []string, runner tuiSpinRunner, spin tuiSpinAction, stdout io.Writer) error {
	spinTitle, _ := cmd.Flags().GetString("title")
	spinType, _ := cmd.Flags().GetString("type")

	if len(args) == 0 {
		return errors.New("no command specified; use -- to separate spin flags from the command")
	}

	// Find the command to run (after --)
	command := args[0]
	cmdArgs := args[1:]
	ctx := cmd.Context()

	// Check if we should delegate to parent TUI server
	if client := tuiclient.NewClientFromEnv(); client != nil {
		if clientErr := client.SpinContext(ctx, tuiwire.SpinRequest{
			Title:   spinTitle,
			Spinner: spinType,
		}); clientErr != nil {
			return clientErr
		}
	}
	parsedType, parseErr := tui.ParseSpinnerType(spinType)
	if parseErr != nil {
		return parseErr
	}

	// The result goes through a buffered channel: the send happens before the
	// action ends, and SpinWithAction returns after it observed the end, so a
	// non-blocking receive sees it on every path where the command ended.
	results := make(chan tuiSpinRunResult, 1)
	spinErr := spin(ctx, tui.SpinOptions{
		Title: spinTitle,
		Type:  parsedType,
	}, func(actionCtx context.Context) {
		results <- runner.Run(actionCtx, command, cmdArgs)
	})
	if errors.Is(spinErr, tui.ErrSpinActionStillRunning) {
		// The command may still write its result: never read it.
		return spinErr
	}

	var runResult tuiSpinRunResult
	select {
	case runResult = <-results:
	default:
		// The command never ran: the terminal could not be resolved.
		if spinErr != nil {
			return spinErr
		}
		return errTUISpinNotRun
	}

	// Print the command output
	if len(runResult.Output) > 0 {
		_, _ = fmt.Fprint(stdout, strings.TrimSuffix(string(runResult.Output), "\n"))
		_, _ = fmt.Fprintln(stdout)
	}

	if errors.Is(spinErr, tui.ErrSpinCancelled) {
		return spinErr
	}
	if spinErr != nil {
		// The spinner failed to draw, but the command ran: its status stands.
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: spinner failed: %v\n", spinErr)
	}
	return tuiSpinCommandError(ctx, cmd, runResult)
}

// tuiSpinCommandError maps the wrapped command's result to the error that
// gives spin its exit status:
//   - a failure after ctx, the cmd context, was cancelled (SIGINT) wraps
//     context.Canceled, status 1;
//   - a runner error is returned as is, status 1;
//   - a status outside 0-255 (-1 when a signal killed the command) is an
//     error with status 1, as a native `invowk cmd` gives;
//   - any other non-zero status is forwarded through ExitError.
func tuiSpinCommandError(ctx context.Context, cmd *cobra.Command, result tuiSpinRunResult) error {
	failed := result.Err != nil || result.ExitCode != 0
	if ctxErr := ctx.Err(); ctxErr != nil && failed {
		return fmt.Errorf("spin: command cancelled: %w", ctxErr)
	}
	if result.Err != nil {
		return result.Err
	}
	if result.ExitCode < 0 {
		return fmt.Errorf("spin: command ended by a signal (exit status %d)", result.ExitCode)
	}
	if err := result.ExitCode.Validate(); err != nil {
		return fmt.Errorf("spin: command exit status: %w", err)
	}
	if result.ExitCode != 0 {
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		return &ExitError{Code: result.ExitCode}
	}
	return nil
}

func (r tuiSpinRunResult) Validate() error {
	return r.ExitCode.Validate()
}

//goplint:ignore -- exec.CommandContext receives user command argv from Cobra.
func (execTUISpinRunner) Run(ctx context.Context, command string, args []string) tuiSpinRunResult {
	execCmd := exec.CommandContext(ctx, command, args...)
	execCmd.WaitDelay = tuiSpinCommandWaitDelay
	output, err := execCmd.CombinedOutput()
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return tuiSpinRunResult{
			Output:   output,
			ExitCode: types.ExitCode(exitErr.ExitCode()), //goplint:ignore -- OS exit code from exec.ExitError
		}
	}
	return tuiSpinRunResult{Output: output, Err: err}
}
