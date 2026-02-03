// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"invowk-cli/internal/uroot"
	"invowk-cli/pkg/invkfile"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// VirtualRuntime executes commands using mvdan/sh with optional u-root utilities
type VirtualRuntime struct {
	// EnableUrootUtils enables u-root built-in utilities
	EnableUrootUtils bool
}

// NewVirtualRuntime creates a new virtual runtime
func NewVirtualRuntime(enableUroot bool) *VirtualRuntime {
	return &VirtualRuntime{
		EnableUrootUtils: enableUroot,
	}
}

// Name returns the runtime name
func (r *VirtualRuntime) Name() string {
	return "virtual"
}

// Available returns whether this runtime is available
func (r *VirtualRuntime) Available() bool {
	// Virtual runtime is always available as it's built-in
	return true
}

// Validate checks if a command can be executed
func (r *VirtualRuntime) Validate(ctx *ExecutionContext) error {
	if ctx.SelectedImpl == nil {
		return fmt.Errorf("no script selected for execution")
	}
	if ctx.SelectedImpl.Script == "" {
		return fmt.Errorf("script has no content to execute")
	}

	// Check if interpreter is configured (not allowed for virtual runtime)
	// This is a Go-level validation in addition to CUE schema validation
	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig != nil {
		if err := rtConfig.ValidateInterpreterForRuntime(); err != nil {
			return err
		}
	}

	// Resolve the script content
	script, err := ctx.SelectedImpl.ResolveScript(ctx.Invkfile.FilePath)
	if err != nil {
		return err
	}

	// Try to parse the script to validate syntax
	_, err = syntax.NewParser().Parse(strings.NewReader(script), "script")
	if err != nil {
		return fmt.Errorf("script syntax error: %w", err)
	}

	return nil
}

// Execute runs a command using the virtual shell
func (r *VirtualRuntime) Execute(ctx *ExecutionContext) *Result {
	// Validate interpreter is not set (virtual runtime doesn't support custom interpreters)
	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig != nil {
		if err := rtConfig.ValidateInterpreterForRuntime(); err != nil {
			return &Result{ExitCode: 1, Error: err}
		}
	}

	// Resolve the script content
	script, err := ctx.SelectedImpl.ResolveScript(ctx.Invkfile.FilePath)
	if err != nil {
		return &Result{ExitCode: 1, Error: err}
	}

	// Parse the script
	parser := syntax.NewParser()
	prog, err := parser.Parse(strings.NewReader(script), "script")
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("failed to parse script: %w", err)}
	}

	// Determine working directory
	workDir := r.getWorkDir(ctx)

	// Build environment
	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritAll)
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("failed to build environment: %w", err)}
	}

	// Create the interpreter
	opts := []interp.RunnerOption{
		interp.Dir(workDir),
		interp.Env(expand.ListEnviron(EnvToSlice(env)...)),
		interp.StdIO(ctx.IO.Stdin, ctx.IO.Stdout, ctx.IO.Stderr),
		interp.ExecHandlers(r.execHandler),
	}

	// Add positional parameters for shell access ($1, $2, etc.)
	// Prepend "--" to signal end of options; without this, args like "-v" or "--env=staging"
	// are incorrectly interpreted as shell options by interp.Params()
	if len(ctx.PositionalArgs) > 0 {
		params := append([]string{"--"}, ctx.PositionalArgs...)
		opts = append(opts, interp.Params(params...))
	}

	runner, err := interp.New(opts...)
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("failed to create interpreter: %w", err)}
	}

	// Execute
	execCtx := ctx.Context
	if execCtx == nil {
		execCtx = context.Background()
	}

	err = runner.Run(execCtx, prog)
	if err != nil {
		var exitStatus interp.ExitStatus
		if errors.As(err, &exitStatus) {
			return &Result{ExitCode: int(exitStatus)}
		}
		return &Result{ExitCode: 1, Error: fmt.Errorf("script execution failed: %w", err)}
	}

	return &Result{ExitCode: 0}
}

// ExecuteCapture runs a command and captures its output
func (r *VirtualRuntime) ExecuteCapture(ctx *ExecutionContext) *Result {
	// Resolve the script content
	script, err := ctx.SelectedImpl.ResolveScript(ctx.Invkfile.FilePath)
	if err != nil {
		return &Result{ExitCode: 1, Error: err}
	}

	parser := syntax.NewParser()
	prog, err := parser.Parse(strings.NewReader(script), "script")
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("failed to parse script: %w", err)}
	}

	workDir := r.getWorkDir(ctx)
	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritAll)
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("failed to build environment: %w", err)}
	}

	var stdout, stderr bytes.Buffer

	opts := []interp.RunnerOption{
		interp.Dir(workDir),
		interp.Env(expand.ListEnviron(EnvToSlice(env)...)),
		interp.StdIO(nil, &stdout, &stderr),
		interp.ExecHandlers(r.execHandler),
	}

	// Add positional parameters for shell access ($1, $2, etc.)
	// Prepend "--" to signal end of options; without this, args like "-v" or "--env=staging"
	// are incorrectly interpreted as shell options by interp.Params()
	if len(ctx.PositionalArgs) > 0 {
		params := append([]string{"--"}, ctx.PositionalArgs...)
		opts = append(opts, interp.Params(params...))
	}

	runner, err := interp.New(opts...)
	if err != nil {
		return &Result{ExitCode: 1, Error: fmt.Errorf("failed to create interpreter: %w", err)}
	}

	execCtx := ctx.Context
	if execCtx == nil {
		execCtx = context.Background()
	}

	result := &Result{
		Output:    stdout.String(),
		ErrOutput: stderr.String(),
	}

	err = runner.Run(execCtx, prog)
	if err != nil {
		var exitStatus interp.ExitStatus
		if errors.As(err, &exitStatus) {
			result.ExitCode = int(exitStatus)
		} else {
			result.ExitCode = 1
			result.Error = err
		}
	}

	result.Output = stdout.String()
	result.ErrOutput = stderr.String()

	return result
}

// SupportsInteractive returns true as the virtual runtime always supports interactive mode.
// Interactive mode is achieved by spawning a subprocess wrapper.
func (r *VirtualRuntime) SupportsInteractive() bool {
	return true
}

// PrepareInteractive prepares the virtual runtime for interactive execution.
// This is an alias for PrepareCommand to implement the InteractiveRuntime interface.
func (r *VirtualRuntime) PrepareInteractive(ctx *ExecutionContext) (*PreparedCommand, error) {
	return r.PrepareCommand(ctx)
}

// PrepareCommand prepares the virtual execution for interactive mode.
// Since mvdan/sh runs entirely in-process, we spawn a subprocess that can be
// attached to a PTY. The subprocess invokes `invowk internal exec-virtual`
// which executes the virtual shell with PTY stdio.
func (r *VirtualRuntime) PrepareCommand(ctx *ExecutionContext) (*PreparedCommand, error) {
	// Validate interpreter is not set (virtual runtime doesn't support custom interpreters)
	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig != nil {
		if err := rtConfig.ValidateInterpreterForRuntime(); err != nil {
			return nil, err
		}
	}

	// Resolve the script content
	script, err := ctx.SelectedImpl.ResolveScript(ctx.Invkfile.FilePath)
	if err != nil {
		return nil, err
	}

	// Validate script syntax before creating subprocess
	_, err = syntax.NewParser().Parse(strings.NewReader(script), "script")
	if err != nil {
		return nil, fmt.Errorf("script syntax error: %w", err)
	}

	// Create temp file for script
	tmpFile, err := os.CreateTemp("", "invowk-virtual-*.sh")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp script file: %w", err)
	}

	if _, err = tmpFile.WriteString(script); err != nil {
		_ = tmpFile.Close()           // Best-effort close on error path
		_ = os.Remove(tmpFile.Name()) // Best-effort cleanup on error path
		return nil, fmt.Errorf("failed to write temp script: %w", err)
	}

	if err = tmpFile.Close(); err != nil {
		_ = os.Remove(tmpFile.Name()) // Best-effort cleanup on error path
		return nil, fmt.Errorf("failed to close temp script: %w", err)
	}

	// Get current invowk binary path
	invowkPath, err := os.Executable()
	if err != nil {
		_ = os.Remove(tmpFile.Name()) // Best-effort cleanup on error path
		return nil, fmt.Errorf("failed to get invowk executable path: %w", err)
	}

	// Determine working directory
	workDir := r.getWorkDir(ctx)

	// Build environment
	env, err := buildRuntimeEnv(ctx, invkfile.EnvInheritAll)
	if err != nil {
		_ = os.Remove(tmpFile.Name()) // Best-effort cleanup on error path
		return nil, fmt.Errorf("failed to build environment: %w", err)
	}

	// Serialize environment to JSON for passing to subprocess
	envJSON, err := json.Marshal(env)
	if err != nil {
		_ = os.Remove(tmpFile.Name()) // Best-effort cleanup on error path
		return nil, fmt.Errorf("failed to serialize environment: %w", err)
	}

	// Build subprocess command arguments
	args := []string{
		"internal", "exec-virtual",
		"--script-file", tmpFile.Name(),
		"--workdir", workDir,
		"--env-json", string(envJSON),
	}

	// Add positional arguments
	for _, arg := range ctx.PositionalArgs {
		args = append(args, "--args", arg)
	}

	cmd := exec.CommandContext(ctx.Context, invowkPath, args...)

	// Subprocess inherits filtered environment (TUI server vars will be added by caller)
	cmd.Env = FilterInvowkEnvVars(os.Environ())

	// Track temp file for cleanup
	tempFilePath := tmpFile.Name()
	cleanup := func() {
		_ = os.Remove(tempFilePath) // Cleanup temp file; error non-critical
	}

	return &PreparedCommand{Cmd: cmd, Cleanup: cleanup}, nil
}

// execHandler handles external command execution
func (r *VirtualRuntime) execHandler(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(ctx context.Context, args []string) error {
		// First try u-root builtins if enabled
		if r.EnableUrootUtils {
			if handled, err := r.tryUrootBuiltin(ctx, args); handled {
				return err
			}
		}

		// Fall back to default handler (external commands)
		return next(ctx, args)
	}
}

// tryUrootBuiltin attempts to handle a command with u-root builtins.
//
// Return semantics (User Story 3 - Gradual Adoption with Fallback):
//   - (true, nil): Command was handled successfully by u-root
//   - (true, err): Command was handled by u-root but failed - error is propagated,
//     NO fallback to system binary (error prefixed with [uroot])
//   - (false, nil): Command is not registered in u-root registry - caller should
//     fall back to system binary (enables gradual adoption)
//
// This design ensures:
// 1. Unregistered commands (git, curl, etc.) transparently use system binaries
// 2. Registered commands that fail return errors - no silent fallback that could
//
//	mask implementation bugs or create confusing behavior
func (r *VirtualRuntime) tryUrootBuiltin(ctx context.Context, args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	cmdName := args[0]

	// Look up the command in the u-root registry
	// [US3-T046] Unregistered commands: Lookup returns (nil, false), we return (false, nil)
	// This signals the caller to fall back to system binaries via next() handler
	cmd, found := uroot.DefaultRegistry.Lookup(cmdName)
	if !found {
		return false, nil
	}

	// Execute the u-root command
	// [US3-T047] If the command fails, we return (true, err) - the error is propagated
	// directly to the caller, NO fallback to system binary occurs
	// The command receives the full args slice (including args[0] = command name)
	err := cmd.Run(ctx, args)
	return true, err
}

// getWorkDir determines the working directory using the hierarchical override model.
// Precedence (highest to lowest): CLI override > Implementation > Command > Root > Default
func (r *VirtualRuntime) getWorkDir(ctx *ExecutionContext) string {
	return ctx.Invkfile.GetEffectiveWorkDir(ctx.Command, ctx.SelectedImpl, ctx.WorkDir)
}
