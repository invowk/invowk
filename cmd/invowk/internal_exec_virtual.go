// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// internalExecVirtualCmd executes a script using the virtual runtime.
// This is an internal command used for interactive mode, where the parent
// process needs to attach the execution to a PTY.
//
// The virtual runtime (mvdan/sh) runs entirely in-process, so we need
// a subprocess wrapper to enable PTY attachment.
var internalExecVirtualCmd = &cobra.Command{
	Use:    "exec-virtual",
	Short:  "Execute a script using virtual runtime (internal use only)",
	Hidden: true,
	RunE:   runInternalExecVirtual,
}

func init() {
	internalExecVirtualCmd.Flags().String("script-file", "", "path to script file to execute")
	internalExecVirtualCmd.Flags().String("workdir", "", "working directory for execution")
	internalExecVirtualCmd.Flags().StringArray("env", nil, "environment variables (KEY=VALUE format)")
	internalExecVirtualCmd.Flags().StringArray("args", nil, "positional arguments for the script")
	internalExecVirtualCmd.Flags().String("env-json", "", "environment variables as JSON object")

	_ = internalExecVirtualCmd.MarkFlagRequired("script-file")

	internalCmd.AddCommand(internalExecVirtualCmd)
}

// runInternalExecVirtual executes the virtual shell script.
// It reads the script from the specified file and executes it using mvdan/sh
// with stdin/stdout/stderr connected to the process's stdio (which will be
// attached to a PTY by the parent process).
func runInternalExecVirtual(cmd *cobra.Command, args []string) error {
	scriptFile, _ := cmd.Flags().GetString("script-file")
	workdir, _ := cmd.Flags().GetString("workdir")
	envVars, _ := cmd.Flags().GetStringArray("env")
	posArgs, _ := cmd.Flags().GetStringArray("args")
	envJSON, _ := cmd.Flags().GetString("env-json")

	// Read script content from file
	scriptContent, err := os.ReadFile(scriptFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading script file: %v\n", err)
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		return &ExitError{Code: 1}
	}

	// Parse the script
	parser := syntax.NewParser()
	prog, err := parser.Parse(strings.NewReader(string(scriptContent)), scriptFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing script: %v\n", err)
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		return &ExitError{Code: 1}
	}

	// Build environment
	env, err := buildVirtualEnv(envVars, envJSON)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing environment JSON: %v\n", err)
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		return &ExitError{Code: 1}
	}

	// Create interpreter options
	opts := []interp.RunnerOption{
		interp.StdIO(os.Stdin, os.Stdout, os.Stderr),
		interp.Env(expand.ListEnviron(env...)),
	}

	// Set working directory if specified
	if workdir != "" {
		opts = append(opts, interp.Dir(workdir))
	}

	// Add positional parameters for shell access ($1, $2, etc.)
	// Prepend "--" to signal end of options; without this, args like "-v" or "--env=staging"
	// are incorrectly interpreted as shell options by interp.Params()
	if len(posArgs) > 0 {
		params := append([]string{"--"}, posArgs...)
		opts = append(opts, interp.Params(params...))
	}

	// Create the interpreter
	runner, err := interp.New(opts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating interpreter: %v\n", err)
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		return &ExitError{Code: 1}
	}

	// Execute the script
	ctx := context.Background()
	if err := runner.Run(ctx, prog); err != nil {
		var exitStatus interp.ExitStatus
		if errors.As(err, &exitStatus) {
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			return &ExitError{Code: int(exitStatus)}
		}
		fmt.Fprintf(os.Stderr, "Error executing script: %v\n", err)
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		return &ExitError{Code: 1}
	}

	return nil
}

// buildVirtualEnv builds the environment variable slice from flags and JSON.
// It inherits the current process environment and overlays the provided values.
func buildVirtualEnv(envVars []string, envJSON string) ([]string, error) {
	// Start with current environment
	env := os.Environ()

	// Add env vars from --env flags
	env = append(env, envVars...)

	// Add env vars from --env-json (JSON object format)
	if envJSON != "" {
		var envMap map[string]string
		if err := json.Unmarshal([]byte(envJSON), &envMap); err != nil {
			return nil, err
		}
		for k, v := range envMap {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	return env, nil
}
