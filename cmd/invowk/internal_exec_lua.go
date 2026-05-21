// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"fmt"
	"os"

	ivkruntime "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"

	"github.com/spf13/cobra"
)

// newInternalExecLuaCommand creates the `invowk internal exec-virtual-lua` command.
// This is an internal command used for virtual-lua interactive mode, where the
// parent process needs to attach execution to a PTY.
func newInternalExecLuaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "exec-virtual-lua",
		Short:  "Execute a script using virtual-lua runtime (internal use only)",
		Hidden: true,
		RunE:   runInternalExecLua,
	}

	cmd.Flags().String(flagScriptFile, "", "path to Lua script file to execute")
	cmd.Flags().String("workdir", "", "working directory for execution")
	cmd.Flags().String("script-base-path", "", "script base path for virtual path policy")
	cmd.Flags().StringArray("env", nil, "environment variables (KEY=VALUE format)")
	cmd.Flags().StringArray("args", nil, "positional arguments for the script")
	cmd.Flags().String("env-json", "", "environment variables as JSON object")
	cmd.Flags().StringArray("allowed-binary", nil, "host binary allowed by the virtual runtime")
	cmd.Flags().String("binary-lookup-mode", invowkfile.BinaryLookupModeHost.String(), "host binary lookup mode")
	cmd.Flags().String("filesystem-access", invowkfile.VirtualFilesystemAccessRestricted.String(), "virtual filesystem access mode")
	cmd.Flags().String("filesystem-paths-json", "{}", "virtual filesystem paths as JSON object")
	cmd.Flags().Uint64("cpu-limit", 0, "golua CPU quota")
	cmd.Flags().String("memory-limit", "", "golua memory quota")
	cmd.Flags().Bool("enable-uroot", false, "enable u-root utilities")

	_ = cmd.MarkFlagRequired(flagScriptFile)

	return cmd
}

// runInternalExecLua executes the virtual-lua script with stdin/stdout/stderr
// connected to the process stdio, which the parent process attaches to a PTY.
//
//goplint:ignore -- Cobra adapter receives raw argv/flag strings and validates before delegating to runtime.
func runInternalExecLua(cmd *cobra.Command, _ []string) error {
	scriptFile, _ := cmd.Flags().GetString(flagScriptFile)
	workdir, _ := cmd.Flags().GetString("workdir")
	scriptBasePath, _ := cmd.Flags().GetString("script-base-path")
	envVars, _ := cmd.Flags().GetStringArray("env")
	posArgs, _ := cmd.Flags().GetStringArray("args")
	envJSON, _ := cmd.Flags().GetString("env-json")
	allowedBinaries, _ := cmd.Flags().GetStringArray("allowed-binary")
	binaryLookupModeRaw, _ := cmd.Flags().GetString("binary-lookup-mode")
	filesystemAccessRaw, _ := cmd.Flags().GetString("filesystem-access")
	filesystemPathsRaw, _ := cmd.Flags().GetString("filesystem-paths-json")
	cpuLimit, err := cmd.Flags().GetUint64("cpu-limit")
	if err != nil {
		return internalExecLuaExit(cmd, "Error reading Lua CPU limit: %v\n", err)
	}
	memoryLimitRaw, _ := cmd.Flags().GetString("memory-limit")
	enableUroot, _ := cmd.Flags().GetBool("enable-uroot")
	binaryLookupMode := invowkfile.BinaryLookupMode(binaryLookupModeRaw)
	if validateErr := binaryLookupMode.Validate(); validateErr != nil {
		return internalExecLuaExit(cmd, "Error parsing binary lookup mode: %v\n", validateErr)
	}
	filesystemAccess := invowkfile.VirtualFilesystemAccess(filesystemAccessRaw)
	if validateErr := filesystemAccess.Validate(); validateErr != nil {
		return internalExecLuaExit(cmd, "Error parsing filesystem access: %v\n", validateErr)
	}
	filesystemPaths, err := parseVirtualFilesystemPathsJSON(filesystemPathsRaw)
	if err != nil {
		return internalExecLuaExit(cmd, "Error parsing filesystem paths: %v\n", err)
	}
	luaCPULimit := invowkfile.LuaCPULimit(cpuLimit)
	if validateErr := luaCPULimit.Validate(); validateErr != nil {
		return internalExecLuaExit(cmd, "Error parsing Lua CPU limit: %v\n", validateErr)
	}
	luaMemoryLimit := invowkfile.MemoryLimit(memoryLimitRaw)
	if validateErr := luaMemoryLimit.Validate(); validateErr != nil {
		return internalExecLuaExit(cmd, "Error parsing Lua memory limit: %v\n", validateErr)
	}

	scriptContent, err := os.ReadFile(scriptFile)
	if err != nil {
		return internalExecLuaExit(cmd, "Error reading Lua script file: %v\n", err)
	}

	env, err := buildShEnv(envVars, envJSON)
	if err != nil {
		return internalExecLuaExit(cmd, "Error parsing environment JSON: %v\n", err)
	}

	err = ivkruntime.RunLuaScript(cmd.Context(), ivkruntime.LuaScriptOptions{
		Script:           string(scriptContent),
		ScriptName:       scriptFile,
		WorkDir:          workdir,
		ScriptBasePath:   scriptBasePath,
		Env:              env,
		Args:             posArgs,
		AllowedBinaries:  allowedBinaries,
		BinaryLookupMode: binaryLookupMode,
		FilesystemAccess: filesystemAccess,
		FilesystemPaths:  filesystemPaths,
		CPULimit:         luaCPULimit,
		MemoryLimit:      luaMemoryLimit,
		EnableUroot:      enableUroot,
		Stdin:            os.Stdin,
		Stdout:           os.Stdout,
		Stderr:           os.Stderr,
	})
	if err != nil {
		return internalExecLuaExit(cmd, "Error executing Lua script: %v\n", err)
	}

	return nil
}

//goplint:ignore -- internal CLI adapter renders formatted process-boundary errors to stderr.
func internalExecLuaExit(cmd *cobra.Command, format string, args ...any) error {
	fmt.Fprintf(os.Stderr, format, args...)
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	return &ExitError{Code: 1}
}
