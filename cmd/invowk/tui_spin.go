// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/invowk/invowk/internal/tui"
	"github.com/invowk/invowk/internal/tuiserver"
	"github.com/invowk/invowk/pkg/types"

	"github.com/spf13/cobra"
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

	var output []byte
	var err error
	spinErr := tui.SpinWithAction(tui.SpinOptions{
		Title: spinTitle,
		Type:  parsedType,
	}, func() {
		execCmd := exec.CommandContext(cmd.Context(), command, cmdArgs...)
		output, err = execCmd.CombinedOutput()
	})
	if spinErr != nil {
		return spinErr
	}

	// Print the command output
	if len(output) > 0 {
		_, _ = fmt.Fprint(os.Stdout, strings.TrimSuffix(string(output), "\n"))
		_, _ = fmt.Fprintln(os.Stdout)
	}

	if err != nil {
		// If it's an exec.ExitError, exit with the same code
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			spinExitCode := types.ExitCode(exitErr.ExitCode()) //goplint:ignore -- OS exit code from exec.ExitError
			return &ExitError{Code: spinExitCode}
		}
		return err
	}

	return nil
}
