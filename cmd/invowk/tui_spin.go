// SPDX-License-Identifier: EPL-2.0

package cmd

import (
	"errors"
	"fmt"
	"invowk-cli/internal/tui"
	"invowk-cli/internal/tuiserver"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var (
	spinTitle string
	spinType  string
)

// tuiSpinCmd displays a spinner while running a command.
var tuiSpinCmd = &cobra.Command{
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

func init() {
	tuiCmd.AddCommand(tuiSpinCmd)

	tuiSpinCmd.Flags().StringVar(&spinTitle, "title", "Loading...", "text displayed next to the spinner")
	tuiSpinCmd.Flags().StringVar(&spinType, "type", "line", "spinner animation type")
}

func runTuiSpin(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command specified; use -- to separate spin flags from the command")
	}

	// Find the command to run (after --)
	command := args[0]
	cmdArgs := args[1:]

	var output []byte
	var err error
	var exitCode int

	// Check if we should delegate to parent TUI server
	if client := tuiserver.NewClientFromEnv(); client != nil {
		result, clientErr := client.Spin(tuiserver.SpinRequest{
			Title:   spinTitle,
			Spinner: spinType,
			Command: args, // Full command including args
		})
		if clientErr != nil {
			return clientErr
		}
		output = []byte(result.Stdout)
		exitCode = result.ExitCode
		if exitCode != 0 {
			// Create a synthetic error for non-zero exit
			err = fmt.Errorf("command exited with code %d", exitCode)
		}
	} else {
		// Render TUI directly
		output, err = tui.SpinWithCommand(tui.SpinOptions{
			Title: spinTitle,
			Type:  tui.ParseSpinnerType(spinType),
		}, command, cmdArgs...)
	}

	// Print the command output
	if len(output) > 0 {
		_, _ = fmt.Fprint(os.Stdout, strings.TrimSuffix(string(output), "\n"))
		_, _ = fmt.Fprintln(os.Stdout)
	}

	if err != nil {
		// If it's an exec.ExitError, exit with the same code
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			return &ExitError{Code: exitErr.ExitCode()}
		}
		// If we got a synthetic error from HTTP client
		if exitCode != 0 {
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			return &ExitError{Code: exitCode}
		}
		return err
	}

	return nil
}
