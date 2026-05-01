// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiserver"
	"github.com/invowk/invowk/pkg/types"

	"github.com/spf13/cobra"
)

type (
	tuiSpinRunner interface {
		Run(context.Context, string, []string) tuiSpinRunResult
	}

	tuiSpinAction func(tui.SpinOptions, func()) error

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
	return runTuiSpinWithRunner(cmd, args, execTUISpinRunner{}, tui.SpinWithAction, os.Stdout)
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

	// Check if we should delegate to parent TUI server
	if client := tuiserver.NewClientFromEnv(); client != nil {
		if _, clientErr := client.SpinContext(cmd.Context(), tuiserver.SpinRequest{
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

	var runResult tuiSpinRunResult
	spinErr := spin(tui.SpinOptions{
		Title: spinTitle,
		Type:  parsedType,
	}, func() {
		runResult = runner.Run(cmd.Context(), command, cmdArgs)
	})
	if spinErr != nil {
		return spinErr
	}

	// Print the command output
	if len(runResult.Output) > 0 {
		_, _ = fmt.Fprint(stdout, strings.TrimSuffix(string(runResult.Output), "\n"))
		_, _ = fmt.Fprintln(stdout)
	}

	if runResult.Err != nil {
		return runResult.Err
	}
	if runResult.ExitCode != 0 {
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		return &ExitError{Code: runResult.ExitCode}
	}

	return nil
}

func (r tuiSpinRunResult) Validate() error {
	return r.ExitCode.Validate()
}

//goplint:ignore -- exec.CommandContext receives user command argv from Cobra.
func (execTUISpinRunner) Run(ctx context.Context, command string, args []string) tuiSpinRunResult {
	execCmd := exec.CommandContext(ctx, command, args...)
	output, err := execCmd.CombinedOutput()
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return tuiSpinRunResult{
			Output:   output,
			ExitCode: types.ExitCode(exitErr.ExitCode()), //goplint:ignore -- OS exit code from exec.ExitError
		}
	}
	return tuiSpinRunResult{Output: output, Err: err}
}
