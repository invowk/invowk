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

	"github.com/invowk/invowk/internal/uroot"
	"github.com/invowk/invowk/pkg/invowkfile"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

type (
	// VirtualRuntime executes commands using mvdan/sh with optional u-root utilities.
	// The enableUrootUtils flag is immutable after construction via NewVirtualRuntime.
	VirtualRuntime struct {
		//plint:internal -- required constructor param; immutable after construction
		enableUrootUtils bool
		//plint:internal -- has WithVirtualEnvBuilder(); field name doesn't match pattern
		envBuilder EnvBuilder
		// urootRegistry holds the u-root command registry for built-in utilities.
		// Nil when enableUrootUtils is false.
		urootRegistry *uroot.Registry
	}

	// VirtualRuntimeOption configures a VirtualRuntime.
	VirtualRuntimeOption func(*VirtualRuntime)

	// virtualPreparedExec holds a parsed program and runner ready for execution.
	// This bundles the result of script resolution, parsing, env building, and
	// runner creation shared between Execute and ExecuteCapture.
	virtualPreparedExec struct {
		prog   *syntax.File
		runner *interp.Runner
	}

	// VirtualScriptOptions configures direct virtual shell execution for the
	// internal interactive subprocess wrapper.
	//goplint:ignore -- subprocess adapter DTO carries shell script text, argv, and env strings across the CLI boundary.
	VirtualScriptOptions struct {
		Script      string
		ScriptName  string
		WorkDir     string
		Env         []string
		Args        []string
		EnableUroot bool
		Stdin       *os.File
		Stdout      *os.File
		Stderr      *os.File
	}
)

// WithVirtualEnvBuilder sets the environment builder for the virtual runtime.
// If not set, NewDefaultEnvBuilder() is used.
func WithVirtualEnvBuilder(b EnvBuilder) VirtualRuntimeOption {
	return func(r *VirtualRuntime) {
		r.envBuilder = b
	}
}

// WithUrootRegistry sets the u-root command registry for the virtual runtime.
// If not set and enableUroot is true, BuildDefaultRegistry() is used.
func WithUrootRegistry(reg *uroot.Registry) VirtualRuntimeOption {
	return func(r *VirtualRuntime) {
		r.urootRegistry = reg
	}
}

// NewVirtualRuntime creates a new virtual runtime with optional configuration.
func NewVirtualRuntime(enableUroot bool, opts ...VirtualRuntimeOption) *VirtualRuntime {
	r := &VirtualRuntime{
		enableUrootUtils: enableUroot,
		envBuilder:       NewDefaultEnvBuilder(),
	}
	for _, opt := range opts {
		opt(r)
	}
	// Default to BuildDefaultRegistry when uroot is enabled and no registry was injected.
	if r.enableUrootUtils && r.urootRegistry == nil {
		r.urootRegistry = uroot.BuildDefaultRegistry()
	}
	return r
}

// UrootUtilsEnabled returns whether u-root utilities are enabled.
func (r *VirtualRuntime) UrootUtilsEnabled() bool { return r.enableUrootUtils }

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
		return errVirtualNoImpl
	}
	if ctx.SelectedImpl.Script == "" {
		return errVirtualNoScript
	}

	if err := validateVirtualInterpreter(ctx); err != nil {
		return err
	}

	// Resolve the script content
	script, err := ctx.ResolveSelectedScript()
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
	if err := validateExecutionContextForRun(ctx, errVirtualNoImpl, errVirtualNoScript); err != nil {
		return NewErrorResult(1, err)
	}

	prepared, execCtx, errResult := r.prepareVirtualExec(ctx, interp.StdIO(ctx.IO.Stdin, ctx.IO.Stdout, ctx.IO.Stderr))
	if errResult != nil {
		return errResult
	}

	err := prepared.runner.Run(execCtx, prepared.prog)
	if err != nil {
		if exitStatus, ok := errors.AsType[interp.ExitStatus](err); ok {
			return NewExitCodeResult(ExitCode(exitStatus))
		}
		return NewErrorResult(1, fmt.Errorf("script execution failed: %w", err))
	}

	return NewSuccessResult()
}

// ExecuteCapture runs a command and captures its output
func (r *VirtualRuntime) ExecuteCapture(ctx *ExecutionContext) *Result {
	if err := validateExecutionContextForRun(ctx, errVirtualNoImpl, errVirtualNoScript); err != nil {
		return NewErrorResult(1, err)
	}

	var stdout, stderr bytes.Buffer

	prepared, execCtx, errResult := r.prepareVirtualExec(ctx, interp.StdIO(nil, &stdout, &stderr))
	if errResult != nil {
		return errResult
	}

	result := &Result{}

	err := prepared.runner.Run(execCtx, prepared.prog)
	if err != nil {
		if exitStatus, ok := errors.AsType[interp.ExitStatus](err); ok {
			result.ExitCode = ExitCode(exitStatus)
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
	if err := validateExecutionContextForRun(ctx, errVirtualNoImpl, errVirtualNoScript); err != nil {
		return nil, err
	}

	if err := validateVirtualInterpreter(ctx); err != nil {
		return nil, err
	}

	// Resolve the script content
	script, err := ctx.ResolveSelectedScript()
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
	workDir := ctx.EffectiveWorkDir()

	// Build environment
	env, err := r.envBuilder.Build(ctx, invowkfile.EnvInheritAll)
	if err != nil {
		_ = os.Remove(tmpFile.Name()) // Best-effort cleanup on error path
		return nil, fmt.Errorf(failedBuildEnvironmentFmt, err)
	}
	ctx.AddTUIEnv(env)

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
	if r.enableUrootUtils {
		args = append(args, "--enable-uroot")
	}

	// Add positional arguments
	for _, arg := range ctx.PositionalArgs {
		args = append(args, "--args", arg)
	}

	cmd := exec.CommandContext(ctx.Context, invowkPath, args...)

	// Subprocess inherits filtered environment; script-visible TUI vars are passed
	// through the serialized environment above.
	cmd.Env = FilterInvowkEnvVars(os.Environ())

	// Track temp file for cleanup
	tempFilePath := tmpFile.Name()
	cleanup := func() {
		_ = os.Remove(tempFilePath) // Cleanup temp file; error non-critical
	}

	return &PreparedCommand{Cmd: cmd, Cleanup: cleanup}, nil
}

// RunVirtualScript executes a virtual shell script with the same u-root command
// handling semantics used by VirtualRuntime. It is used by the internal CLI
// subprocess wrapper for interactive PTY execution.
func RunVirtualScript(ctx context.Context, opts VirtualScriptOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	prog, err := syntax.NewParser().Parse(strings.NewReader(opts.Script), opts.ScriptName)
	if err != nil {
		return fmt.Errorf("parse virtual script: %w", err)
	}

	rt := NewVirtualRuntime(opts.EnableUroot)
	runnerOpts := []interp.RunnerOption{
		interp.StdIO(opts.Stdin, opts.Stdout, opts.Stderr),
		interp.Env(expand.ListEnviron(opts.Env...)),
		interp.ExecHandlers(rt.execHandler),
	}
	if opts.WorkDir != "" {
		runnerOpts = append(runnerOpts, interp.Dir(opts.WorkDir))
	}
	if len(opts.Args) > 0 {
		params := append([]string{"--"}, opts.Args...)
		runnerOpts = append(runnerOpts, interp.Params(params...))
	}

	runner, err := interp.New(runnerOpts...)
	if err != nil {
		return fmt.Errorf("create virtual interpreter: %w", err)
	}
	if err := runner.Run(ctx, prog); err != nil {
		return fmt.Errorf("run virtual script: %w", err)
	}
	return nil
}

// prepareVirtualExec resolves script, parses it, builds environment, and creates
// an interpreter runner. The stdIO option determines whether output is streamed
// or captured. Returns an error Result on failure.
func (r *VirtualRuntime) prepareVirtualExec(ctx *ExecutionContext, stdIO interp.RunnerOption) (*virtualPreparedExec, context.Context, *Result) {
	if err := validateVirtualInterpreter(ctx); err != nil {
		return nil, nil, NewErrorResult(1, err)
	}

	script, err := ctx.ResolveSelectedScript()
	if err != nil {
		return nil, nil, NewErrorResult(1, err)
	}

	prog, err := syntax.NewParser().Parse(strings.NewReader(script), "script")
	if err != nil {
		return nil, nil, NewErrorResult(1, fmt.Errorf("failed to parse script: %w", err))
	}

	env, err := r.envBuilder.Build(ctx, invowkfile.EnvInheritAll)
	if err != nil {
		return nil, nil, NewErrorResult(1, fmt.Errorf(failedBuildEnvironmentFmt, err))
	}
	ctx.AddTUIEnv(env)

	opts := []interp.RunnerOption{
		interp.Dir(ctx.EffectiveWorkDir()),
		interp.Env(expand.ListEnviron(EnvToSlice(env)...)),
		stdIO,
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
		return nil, nil, NewErrorResult(1, fmt.Errorf("failed to create interpreter: %w", err))
	}

	execCtx := ctx.Context
	if execCtx == nil {
		execCtx = context.Background()
	}

	return &virtualPreparedExec{prog: prog, runner: runner}, execCtx, nil
}

func validateVirtualInterpreter(ctx *ExecutionContext) error {
	if ctx == nil || ctx.SelectedImpl == nil {
		return nil
	}
	rtConfig := ctx.SelectedImpl.GetRuntimeConfig(ctx.SelectedRuntime)
	if rtConfig == nil {
		return nil
	}
	return rtConfig.ValidateInterpreterForRuntime()
}

// execHandler handles external command execution
func (r *VirtualRuntime) execHandler(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(ctx context.Context, args []string) error {
		// First try u-root builtins if enabled
		if r.enableUrootUtils {
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

	// Check if the command exists in the u-root registry.
	// [US3-T046] Unregistered commands: Lookup returns (nil, false), we return (false, nil)
	// This signals the caller to fall back to system binaries via next() handler.
	if _, found := r.urootRegistry.Lookup(cmdName); !found {
		return false, nil
	}

	// Execute via Registry.Run for centralized POSIX flag preprocessing.
	// [US3-T047] If the command fails, we return (true, err) - the error is propagated
	// directly to the caller, NO fallback to system binary occurs.
	// Registry.Run splits combined short flags (e.g., "-sf" → "-s", "-f") for custom
	// implementations before dispatching to cmd.Run().
	err := r.urootRegistry.Run(ctx, cmdName, args)
	return true, err
}
