package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"invowk-cli/internal/tui"
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

	output, err := tui.SpinWithCommand(tui.SpinOptions{
		Title: spinTitle,
		Type:  tui.ParseSpinnerType(spinType),
	}, command, cmdArgs...)

	// Print the command output
	if len(output) > 0 {
		fmt.Fprint(os.Stdout, strings.TrimSuffix(string(output), "\n"))
		fmt.Fprintln(os.Stdout)
	}

	if err != nil {
		// If it's an exec.ExitError, exit with the same code
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		return err
	}

	return nil
}
