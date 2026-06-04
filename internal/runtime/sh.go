// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/invowk/invowk/internal/uroot"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

const errVirtualHostBinaryDeniedExitStatus interp.ExitStatus = 126

type (
	// ShRuntime executes commands using mvdan/sh with optional u-root utilities.
	// The enableUrootUtils flag is immutable after construction via NewShRuntime.
	ShRuntime struct {
		//plint:internal -- required constructor param; immutable after construction
		enableUrootUtils bool
		//plint:internal -- has WithShEnvBuilder(); field name doesn't match pattern
		envBuilder EnvBuilder
		// urootRegistry holds the u-root command registry for built-in utilities.
		// Nil when enableUrootUtils is false.
		urootRegistry *uroot.Registry
		// interactiveCommandFactory creates the subprocess used for PTY-backed execution.
		interactiveCommandFactory ShInteractiveCommandFactory
	}

	// ShRuntimeOption configures a ShRuntime.
	ShRuntimeOption func(*ShRuntime)

	// ShInteractiveEnvJSON is the serialized environment passed to the virtual subprocess adapter.
	ShInteractiveEnvJSON string

	// ShInteractiveArgs contains command positional arguments passed to the virtual subprocess adapter.
	ShInteractiveArgs []string

	// ShInteractiveCommandSpec describes a prepared virtual interactive subprocess request.
	//goplint:ignore -- subprocess adapter DTO carries CLI-boundary argv and serialized env.
	ShInteractiveCommandSpec struct {
		ScriptFile       *types.FilesystemPath
		WorkDir          *types.FilesystemPath
		ScriptBasePath   *types.FilesystemPath
		EnvJSON          ShInteractiveEnvJSON
		Args             ShInteractiveArgs
		AllowedBinaries  []string
		BinaryLookupMode invowkfile.BinaryLookupMode
		FilesystemAccess invowkfile.VirtualFilesystemAccess
		FilesystemPaths  invowkfile.VirtualFilesystemPaths
		EnableUroot      bool
	}

	// ShInteractiveCommandFactory creates the subprocess command for virtual interactive execution.
	ShInteractiveCommandFactory func(ctx context.Context, spec ShInteractiveCommandSpec) (*exec.Cmd, error)

	// shPreparedExec holds a parsed program and runner ready for execution.
	// This bundles the result of script resolution, parsing, env building, and
	// runner creation shared between Execute and ExecuteCapture.
	shPreparedExec struct {
		prog   *syntax.File
		runner *interp.Runner
	}

	// ShScriptOptions configures direct virtual shell execution for the
	// internal interactive subprocess wrapper.
	//goplint:ignore -- subprocess adapter DTO carries shell script text, argv, and env strings across the CLI boundary.
	ShScriptOptions struct {
		Script           string
		ScriptName       string
		WorkDir          string
		ScriptBasePath   string
		Env              []string
		Args             []string
		AllowedBinaries  []string
		BinaryLookupMode invowkfile.BinaryLookupMode
		FilesystemAccess invowkfile.VirtualFilesystemAccess
		FilesystemPaths  invowkfile.VirtualFilesystemPaths
		EnableUroot      bool
		Stdin            *os.File
		Stdout           *os.File
		Stderr           *os.File
	}
)

// WithShEnvBuilder sets the environment builder for the virtual runtime.
// If not set, NewDefaultEnvBuilder() is used.
func WithShEnvBuilder(b EnvBuilder) ShRuntimeOption {
	return func(r *ShRuntime) {
		r.envBuilder = b
	}
}

// WithUrootRegistry sets the u-root command registry for the virtual runtime.
// If not set and enableUroot is true, BuildDefaultRegistry() is used.
func WithUrootRegistry(reg *uroot.Registry) ShRuntimeOption {
	return func(r *ShRuntime) {
		r.urootRegistry = reg
	}
}

// WithInteractiveCommandFactory injects the subprocess factory used for PTY-backed virtual execution.
func WithInteractiveCommandFactory(factory ShInteractiveCommandFactory) ShRuntimeOption {
	return func(r *ShRuntime) {
		r.interactiveCommandFactory = factory
	}
}

// String returns the serialized environment JSON.
func (e ShInteractiveEnvJSON) String() string { return string(e) }

// Validate returns an error if the serialized environment is empty.
func (e ShInteractiveEnvJSON) Validate() error {
	if strings.TrimSpace(string(e)) == "" {
		return errors.New("virtual interactive env JSON must be non-empty")
	}
	return nil
}

// Validate returns nil when the subprocess request contains required paths and env data.
func (s ShInteractiveCommandSpec) Validate() error {
	var scriptFileErr error
	if s.ScriptFile == nil {
		scriptFileErr = errors.New("virtual interactive script file is required")
	} else {
		scriptFileErr = s.ScriptFile.Validate()
	}
	var workDirErr error
	if s.WorkDir == nil {
		workDirErr = errors.New("virtual interactive workdir is required")
	} else {
		workDirErr = s.WorkDir.Validate()
	}
	var scriptBaseErr error
	if s.ScriptBasePath == nil {
		scriptBaseErr = errors.New("virtual interactive script base path is required")
	} else {
		scriptBaseErr = s.ScriptBasePath.Validate()
	}
	return errors.Join(
		scriptFileErr,
		workDirErr,
		scriptBaseErr,
		s.EnvJSON.Validate(),
		s.BinaryLookupMode.Validate(),
		s.FilesystemAccess.Validate(),
		s.FilesystemPaths.Validate(),
	)
}

// NewShRuntime creates a new virtual runtime with optional configuration.
func NewShRuntime(enableUroot bool, opts ...ShRuntimeOption) *ShRuntime {
	r := &ShRuntime{
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
func (r *ShRuntime) UrootUtilsEnabled() bool { return r.enableUrootUtils }

// Name returns the runtime name
func (r *ShRuntime) Name() string {
	return RuntimeTypeVirtualSh.String()
}

// Available returns whether this runtime is available
func (r *ShRuntime) Available() bool {
	// Virtual runtime is always available as it's built-in
	return true
}

// Validate checks if a command can be executed
func (r *ShRuntime) Validate(ctx *ExecutionContext) error {
	if ctx.SelectedImpl == nil {
		return errVirtualNoImpl
	}
	if err := ctx.SelectedImpl.Script.Validate(); err != nil {
		return errVirtualNoScript
	}

	// Resolve the script content
	script, err := ctx.ResolveSelectedScript()
	if err != nil {
		return err
	}
	if interpErr := validateShInterpreter(ctx.SelectedImpl.Script, script); interpErr != nil {
		return interpErr
	}

	// Try to parse the script to validate syntax
	_, err = syntax.NewParser().Parse(strings.NewReader(script), "script")
	if err != nil {
		return fmt.Errorf("script syntax error: %w", err)
	}

	return nil
}

// Execute runs a command using the virtual shell
func (r *ShRuntime) Execute(ctx *ExecutionContext) *Result {
	if err := validateExecutionContextForRun(ctx, errVirtualNoImpl, errVirtualNoScript); err != nil {
		return NewErrorResult(1, err)
	}

	prepared, execCtx, errResult := r.prepareShExec(ctx, interp.StdIO(ctx.IO.Stdin, ctx.IO.Stdout, ctx.IO.Stderr))
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
func (r *ShRuntime) ExecuteCapture(ctx *ExecutionContext) *Result {
	if err := validateExecutionContextForRun(ctx, errVirtualNoImpl, errVirtualNoScript); err != nil {
		return NewErrorResult(1, err)
	}

	var stdout, stderr bytes.Buffer

	prepared, execCtx, errResult := r.prepareShExec(ctx, interp.StdIO(nil, &stdout, &stderr))
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
func (r *ShRuntime) SupportsInteractive() bool {
	return true
}

// PrepareInteractive prepares the virtual runtime for interactive execution.
// This is an alias for PrepareCommand to implement the InteractiveRuntime interface.
func (r *ShRuntime) PrepareInteractive(ctx *ExecutionContext) (*PreparedCommand, error) {
	return r.PrepareCommand(ctx)
}

// PrepareCommand prepares the virtual execution for interactive mode.
// Since mvdan/sh runs entirely in-process, we ask the composition adapter to
// create a subprocess that can be attached to a PTY.
func (r *ShRuntime) PrepareCommand(ctx *ExecutionContext) (*PreparedCommand, error) {
	if err := validateExecutionContextForRun(ctx, errVirtualNoImpl, errVirtualNoScript); err != nil {
		return nil, err
	}

	// Resolve the script content
	script, err := ctx.ResolveSelectedScript()
	if err != nil {
		return nil, err
	}
	if interpErr := validateShInterpreter(ctx.SelectedImpl.Script, script); interpErr != nil {
		return nil, interpErr
	}

	// Validate script syntax before creating subprocess
	_, err = syntax.NewParser().Parse(strings.NewReader(script), "script")
	if err != nil {
		return nil, fmt.Errorf("script syntax error: %w", err)
	}

	prepared, err := prepareVirtualInteractiveSubprocess(
		ctx,
		script,
		"invowk-virtual-*.sh",
		"script",
		"virtual interactive",
		r.envBuilder,
	)
	if err != nil {
		return nil, err
	}

	if r.interactiveCommandFactory == nil {
		prepared.cleanup()
		return nil, ErrShInteractiveLauncherNotConfigured
	}

	spec := ShInteractiveCommandSpec{
		ScriptFile:       &prepared.scriptFile,
		WorkDir:          &prepared.workDir,
		ScriptBasePath:   &prepared.scriptBasePath,
		EnvJSON:          ShInteractiveEnvJSON(prepared.envJSON),
		Args:             ShInteractiveArgs(append([]string(nil), ctx.PositionalArgs...)),
		AllowedBinaries:  prepared.allowedBinaries,
		BinaryLookupMode: prepared.binaryLookupMode,
		FilesystemAccess: prepared.filesystemAccess,
		FilesystemPaths:  prepared.filesystemPaths,
		EnableUroot:      r.enableUrootUtils,
	}
	if validateErr := spec.Validate(); validateErr != nil {
		prepared.cleanup()
		return nil, fmt.Errorf("invalid virtual interactive command spec: %w", validateErr)
	}

	cmd, err := r.interactiveCommandFactory(ctx.Context, spec)
	if err != nil {
		prepared.cleanup()
		return nil, fmt.Errorf("create virtual interactive subprocess: %w", err)
	}

	return &PreparedCommand{Cmd: cmd, Cleanup: prepared.cleanup}, nil
}

// RunShScript executes a virtual shell script with the same u-root command
// handling semantics used by ShRuntime. It is used by the internal CLI
// subprocess wrapper for interactive PTY execution.
//
//nolint:contextcheck // nil context is accepted for internal subprocess compatibility.
func RunShScript(ctx context.Context, opts ShScriptOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := opts.FilesystemAccess.Validate(); err != nil {
		return err
	}
	if err := opts.FilesystemPaths.Validate(); err != nil {
		return err
	}
	prog, err := syntax.NewParser().Parse(strings.NewReader(opts.Script), opts.ScriptName)
	if err != nil {
		return fmt.Errorf("parse virtual script: %w", err)
	}

	rt := NewShRuntime(opts.EnableUroot)
	env := SliceToEnv(opts.Env)
	pathResolver, err := newVirtualPathResolverForInteractiveConfig(
		opts.WorkDir,
		opts.ScriptBasePath,
		invowkfile.VirtualFilesystemConfig{
			Access: opts.FilesystemAccess,
			Paths:  opts.FilesystemPaths,
		},
	)
	if err != nil {
		return err
	}
	addVirtualRuntimeEnv(env, pathResolver)
	pathValidator := virtualPathValidator{resolver: pathResolver}
	binaryPolicy := &virtualHostBinaryPolicy{
		allowed:  append([]string(nil), opts.AllowedBinaries...),
		mode:     opts.BinaryLookupMode,
		workDir:  opts.WorkDir,
		envPath:  env["PATH"],
		pathext:  env["PATHEXT"],
		stateEnv: env,
	}
	if binaryPolicy.mode == "" {
		binaryPolicy.mode = invowkfile.BinaryLookupModeHost
	}
	runnerOpts := []interp.RunnerOption{
		interp.StdIO(opts.Stdin, opts.Stdout, opts.Stderr),
		interp.Env(expand.ListEnviron(EnvToSlice(env)...)),
		interp.ExecHandlers(rt.execHandler(binaryPolicy, pathValidator)),
		interp.OpenHandler(pathValidator.openHandler(interp.DefaultOpenHandler())),
		interp.ReadDirHandler2(pathValidator.readDirHandler(interp.DefaultReadDirHandler2())),
		interp.StatHandler(pathValidator.statHandler(interp.DefaultStatHandler())),
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

// prepareShExec resolves script, parses it, builds environment, and creates
// an interpreter runner. The stdIO option determines whether output is streamed
// or captured. Returns an error Result on failure.
func (r *ShRuntime) prepareShExec(ctx *ExecutionContext, stdIO interp.RunnerOption) (*shPreparedExec, context.Context, *Result) {
	script, err := ctx.ResolveSelectedScript()
	if err != nil {
		return nil, nil, NewErrorResult(1, err)
	}
	if interpErr := validateShInterpreter(ctx.SelectedImpl.Script, script); interpErr != nil {
		return nil, nil, NewErrorResult(1, interpErr)
	}

	prog, err := syntax.NewParser().Parse(strings.NewReader(script), "script")
	if err != nil {
		return nil, nil, NewErrorResult(1, fmt.Errorf("failed to parse script: %w", err))
	}

	env, err := r.envBuilder.Build(ctx, invowkfile.EnvInheritAll)
	if err != nil {
		return nil, nil, NewErrorResult(1, fmt.Errorf(failedBuildEnvironmentFmt, err))
	}
	pathValidator, err := newVirtualPathValidator(ctx)
	if err != nil {
		return nil, nil, NewErrorResult(1, err)
	}
	addVirtualRuntimeEnv(env, pathValidator.resolver)
	ctx.AddTUIEnv(env)
	binaryPolicy := hostBinaryPolicy(ctx, env)

	opts := []interp.RunnerOption{
		interp.Dir(ctx.EffectiveWorkDir()),
		interp.Env(expand.ListEnviron(EnvToSlice(env)...)),
		stdIO,
		interp.ExecHandlers(r.execHandler(binaryPolicy, pathValidator)),
		interp.OpenHandler(pathValidator.openHandler(interp.DefaultOpenHandler())),
		interp.ReadDirHandler2(pathValidator.readDirHandler(interp.DefaultReadDirHandler2())),
		interp.StatHandler(pathValidator.statHandler(interp.DefaultStatHandler())),
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

	return &shPreparedExec{prog: prog, runner: runner}, execCtx, nil
}

//goplint:ignore -- virtual runtime validates resolved script text produced by the shared script resolver.
func validateShInterpreter(script invowkfile.ImplementationScript, scriptContent string) error {
	interpInfo := script.ResolveInterpreterFromScript(scriptContent)
	if !interpInfo.Found || invowkfile.IsShellInterpreter(interpInfo.Interpreter) {
		return nil
	}
	return fmt.Errorf("%w (got %q); virtual-sh uses mvdan/sh and cannot execute non-shell interpreters", invowkfile.ErrInterpreterNotAllowed, interpInfo.Interpreter)
}

// execHandler handles external command execution
func (r *ShRuntime) execHandler(policy *virtualHostBinaryPolicy, pathValidator virtualPathValidator) func(interp.ExecHandlerFunc) interp.ExecHandlerFunc {
	return func(next interp.ExecHandlerFunc) interp.ExecHandlerFunc {
		return func(ctx context.Context, args []string) error {
			if r.enableUrootUtils {
				if handled, err := r.tryUrootBuiltin(ctx, args, pathValidator); handled {
					return err
				}
			}

			if len(args) == 0 {
				return nil
			}
			path, err := policy.resolve(args[0])
			if err != nil {
				fmt.Fprintln(interp.HandlerCtx(ctx).Stderr, err)
				return errVirtualHostBinaryDeniedExitStatus
			}
			resolvedArgs := append([]string{path}, args[1:]...)
			return next(ctx, resolvedArgs)
		}
	}
}

// tryUrootBuiltin attempts to handle a command with u-root builtins.
//
// Return semantics:
//   - (true, nil): Command was handled successfully by u-root
//   - (true, err): Command was handled by u-root but failed - error is propagated,
//     and the host-binary policy is not consulted.
//   - (false, nil): Command is not registered in u-root registry; caller should
//     evaluate the virtual host-binary policy.
//
// This design ensures:
// 1. Registered commands that fail return errors - no silent host-binary policy retry that could
//
//	mask implementation bugs or create confusing behavior
func (r *ShRuntime) tryUrootBuiltin(ctx context.Context, args []string, pathValidator virtualPathValidator) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	cmdName := args[0]

	// Check if the command exists in the u-root registry.
	// [US3-T046] Unregistered commands: Lookup returns (nil, false), we return (false, nil)
	// This signals the caller to evaluate the explicit host-binary policy.
	if _, found := r.urootRegistry.Lookup(cmdName); !found {
		return false, nil
	}

	// Execute via Registry.Run for centralized POSIX flag preprocessing.
	// [US3-T047] If the command fails, we return (true, err) - the error is propagated
	// directly to the caller, and the host-binary policy is not consulted.
	// Registry.Run splits combined short flags (e.g., "-sf" → "-s", "-f") for custom
	// implementations before dispatching to cmd.Run().
	handler := uroot.ExtractHandlerContext(ctx)
	handler.ValidatePath = pathValidator.validate
	err := r.urootRegistry.Run(uroot.WithHandlerContext(ctx, handler), cmdName, args)
	return true, err
}
