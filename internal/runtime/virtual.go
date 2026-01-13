// SPDX-License-Identifier: EPL-2.0

package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	env := r.buildEnv(ctx)

	// Create the interpreter
	opts := []interp.RunnerOption{
		interp.Dir(workDir),
		interp.Env(expand.ListEnviron(EnvToSlice(env)...)),
		interp.StdIO(ctx.Stdin, ctx.Stdout, ctx.Stderr),
		interp.ExecHandlers(r.execHandler),
	}

	// Add positional parameters for shell access ($1, $2, etc.)
	if len(ctx.PositionalArgs) > 0 {
		opts = append(opts, interp.Params(ctx.PositionalArgs...))
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
		if status, ok := interp.IsExitStatus(err); ok {
			return &Result{ExitCode: int(status)}
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
	env := r.buildEnv(ctx)

	var stdout, stderr bytes.Buffer

	opts := []interp.RunnerOption{
		interp.Dir(workDir),
		interp.Env(expand.ListEnviron(EnvToSlice(env)...)),
		interp.StdIO(nil, &stdout, &stderr),
		interp.ExecHandlers(r.execHandler),
	}

	// Add positional parameters for shell access ($1, $2, etc.)
	if len(ctx.PositionalArgs) > 0 {
		opts = append(opts, interp.Params(ctx.PositionalArgs...))
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
		if status, ok := interp.IsExitStatus(err); ok {
			result.ExitCode = int(status)
		} else {
			result.ExitCode = 1
			result.Error = err
		}
	}

	result.Output = stdout.String()
	result.ErrOutput = stderr.String()

	return result
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

// tryUrootBuiltin attempts to handle a command with u-root builtins
func (r *VirtualRuntime) tryUrootBuiltin(ctx context.Context, args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	// Core builtins that are commonly needed
	// For now, we delegate to the interp default which uses the system's commands
	// In a full implementation, you would integrate u-root/u-root commands here
	switch args[0] {
	case "echo", "cat", "ls", "pwd", "mkdir", "rm", "cp", "mv", "grep", "head", "tail", "wc", "sort", "uniq", "cut", "tr":
		// These could be implemented with u-root, but for now we use system commands
		return false, nil
	}

	return false, nil
}

// getWorkDir determines the working directory
func (r *VirtualRuntime) getWorkDir(ctx *ExecutionContext) string {
	if ctx.WorkDir != "" {
		return ctx.WorkDir
	}
	if ctx.Command.WorkDir != "" {
		if !filepath.IsAbs(ctx.Command.WorkDir) {
			return filepath.Join(filepath.Dir(ctx.Invkfile.FilePath), ctx.Command.WorkDir)
		}
		return ctx.Command.WorkDir
	}
	return filepath.Dir(ctx.Invkfile.FilePath)
}

// buildEnv builds the environment for the command
func (r *VirtualRuntime) buildEnv(ctx *ExecutionContext) map[string]string {
	env := make(map[string]string)

	// Start with current environment, filtering out Invowk-specific variables
	// to prevent leakage between nested command invocations
	for _, e := range FilterInvowkEnvVars(os.Environ()) {
		if idx := strings.Index(e, "="); idx != -1 {
			env[e[:idx]] = e[idx+1:]
		}
	}

	// Platform-level env from the selected implementation
	currentPlatform := invkfile.GetCurrentHostOS()
	for _, p := range ctx.SelectedImpl.Target.Platforms {
		if p.Name == currentPlatform {
			for k, v := range p.Env {
				env[k] = v
			}
			break
		}
	}

	// Command-level env
	for k, v := range ctx.Command.Env {
		env[k] = v
	}

	// Extra env from context
	for k, v := range ctx.ExtraEnv {
		env[k] = v
	}

	return env
}
