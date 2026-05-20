// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	ivkruntime "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/types"

	"github.com/spf13/cobra"
	"mvdan.cc/sh/v3/interp"
)

const flagScriptFile = "script-file"

// newInternalExecShCommand creates the `invowk internal exec-virtual-sh` command.
// This is an internal command used for virtual-sh interactive mode, where the
// parent process needs to attach execution to a PTY.
func newInternalExecShCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "exec-virtual-sh",
		Short:  "Execute a script using virtual-sh runtime (internal use only)",
		Hidden: true,
		RunE:   runInternalExecSh,
	}

	cmd.Flags().String(flagScriptFile, "", "path to script file to execute")
	cmd.Flags().String("workdir", "", "working directory for execution")
	cmd.Flags().StringArray("env", nil, "environment variables (KEY=VALUE format)")
	cmd.Flags().StringArray("args", nil, "positional arguments for the script")
	cmd.Flags().String("env-json", "", "environment variables as JSON object")
	cmd.Flags().Bool("enable-uroot", false, "enable u-root utilities")

	_ = cmd.MarkFlagRequired(flagScriptFile)

	return cmd
}

// runInternalExecSh executes the virtual-sh script.
// It reads the script from the specified file and executes it using mvdan/sh
// with stdin/stdout/stderr connected to the process's stdio (which will be
// attached to a PTY by the parent process).
func runInternalExecSh(cmd *cobra.Command, _ []string) error {
	scriptFile, _ := cmd.Flags().GetString(flagScriptFile)
	workdir, _ := cmd.Flags().GetString("workdir")
	envVars, _ := cmd.Flags().GetStringArray("env")
	posArgs, _ := cmd.Flags().GetStringArray("args")
	envJSON, _ := cmd.Flags().GetString("env-json")
	enableUroot, _ := cmd.Flags().GetBool("enable-uroot")

	// Read script content from file
	scriptContent, err := os.ReadFile(scriptFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading script file: %v\n", err)
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		return &ExitError{Code: 1}
	}

	// Build environment
	env, err := buildShEnv(envVars, envJSON)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing environment JSON: %v\n", err)
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		return &ExitError{Code: 1}
	}

	// Execute the script
	err = ivkruntime.RunShScript(cmd.Context(), ivkruntime.ShScriptOptions{
		Script:      string(scriptContent),
		ScriptName:  scriptFile,
		WorkDir:     workdir,
		Env:         env,
		Args:        posArgs,
		EnableUroot: enableUroot,
		Stdin:       os.Stdin,
		Stdout:      os.Stdout,
		Stderr:      os.Stderr,
	})
	if err != nil {
		if exitStatus, ok := errors.AsType[interp.ExitStatus](err); ok {
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			return &ExitError{Code: types.ExitCode(exitStatus)}
		}
		fmt.Fprintf(os.Stderr, "Error executing script: %v\n", err)
		cmd.SilenceErrors = true
		cmd.SilenceUsage = true
		return &ExitError{Code: 1}
	}

	return nil
}

// buildShEnv builds the environment variable slice from flags and JSON.
// It inherits the current process environment and overlays the provided values.
func buildShEnv(envVars []string, envJSON string) ([]string, error) {
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
