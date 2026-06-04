// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type (
	// LuaInteractiveEnvJSON is the serialized environment passed to the Lua subprocess adapter.
	LuaInteractiveEnvJSON string

	// LuaInteractiveArgs contains command positional arguments passed to the Lua subprocess adapter.
	LuaInteractiveArgs []string

	// LuaInteractiveCommandSpec describes a prepared virtual-lua interactive subprocess request.
	//goplint:ignore -- subprocess adapter DTO carries CLI-boundary argv and serialized env.
	LuaInteractiveCommandSpec struct {
		ScriptFile       *types.FilesystemPath
		WorkDir          *types.FilesystemPath
		ScriptBasePath   *types.FilesystemPath
		EnvJSON          LuaInteractiveEnvJSON
		Args             LuaInteractiveArgs
		AllowedBinaries  []string
		BinaryLookupMode invowkfile.BinaryLookupMode
		FilesystemAccess invowkfile.VirtualFilesystemAccess
		FilesystemPaths  invowkfile.VirtualFilesystemPaths
		CPULimit         invowkfile.LuaCPULimit
		MemoryLimit      invowkfile.MemoryLimit
		EnableUroot      bool
	}

	// LuaInteractiveCommandFactory creates the subprocess command for virtual-lua interactive execution.
	LuaInteractiveCommandFactory func(ctx context.Context, spec LuaInteractiveCommandSpec) (*exec.Cmd, error)

	// LuaScriptOptions configures direct virtual-lua execution for the internal
	// interactive subprocess wrapper.
	//goplint:ignore -- subprocess adapter DTO carries Lua script text, argv, and env strings across the CLI boundary.
	LuaScriptOptions struct {
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
		CPULimit         invowkfile.LuaCPULimit
		MemoryLimit      invowkfile.MemoryLimit
		EnableUroot      bool
		Stdin            io.Reader
		Stdout           io.Writer
		Stderr           io.Writer
	}
)

// WithLuaInteractiveCommandFactory injects the subprocess factory used for
// PTY-backed virtual-lua execution.
func WithLuaInteractiveCommandFactory(factory LuaInteractiveCommandFactory) LuaRuntimeOption {
	return func(r *LuaRuntime) {
		r.interactiveCommandFactory = factory
	}
}

// String returns the serialized environment JSON.
func (e LuaInteractiveEnvJSON) String() string { return string(e) }

// Validate returns an error if the serialized environment is empty.
func (e LuaInteractiveEnvJSON) Validate() error {
	if strings.TrimSpace(string(e)) == "" {
		return errors.New("virtual-lua interactive env JSON must be non-empty")
	}
	return nil
}

// Validate returns nil when the subprocess request contains required paths and env data.
func (s LuaInteractiveCommandSpec) Validate() error {
	var scriptFileErr error
	if s.ScriptFile == nil {
		scriptFileErr = errors.New("virtual-lua interactive script file is required")
	} else {
		scriptFileErr = s.ScriptFile.Validate()
	}
	var workDirErr error
	if s.WorkDir == nil {
		workDirErr = errors.New("virtual-lua interactive workdir is required")
	} else {
		workDirErr = s.WorkDir.Validate()
	}
	var scriptBaseErr error
	if s.ScriptBasePath == nil {
		scriptBaseErr = errors.New("virtual-lua interactive script base path is required")
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
		s.CPULimit.Validate(),
		s.MemoryLimit.Validate(),
	)
}

// SupportsInteractive returns true as virtual-lua can be executed through a
// subprocess wrapper whose stdio is attached to the caller's PTY.
func (r *LuaRuntime) SupportsInteractive() bool {
	return true
}

// PrepareInteractive prepares the virtual-lua runtime for interactive execution.
func (r *LuaRuntime) PrepareInteractive(ctx *ExecutionContext) (*PreparedCommand, error) {
	return r.PrepareCommand(ctx)
}

// PrepareCommand prepares virtual-lua execution for interactive mode.
func (r *LuaRuntime) PrepareCommand(ctx *ExecutionContext) (*PreparedCommand, error) {
	if err := validateExecutionContextForRun(ctx, errVirtualNoImpl, errVirtualNoScript); err != nil {
		return nil, err
	}

	script, err := ctx.ResolveSelectedScript()
	if err != nil {
		return nil, err
	}
	if interpErr := validateLuaInterpreter(ctx.SelectedImpl.Script, script); interpErr != nil {
		return nil, interpErr
	}
	if _, compileErr := compileLuaChunk(script); compileErr != nil {
		return nil, compileErr
	}

	prepared, err := prepareVirtualInteractiveSubprocess(
		ctx,
		script,
		"invowk-virtual-*.lua",
		"Lua script",
		"virtual-lua interactive",
		r.envBuilder,
	)
	if err != nil {
		return nil, err
	}

	if r.interactiveCommandFactory == nil {
		prepared.cleanup()
		return nil, ErrLuaInteractiveLauncherNotConfigured
	}

	spec := LuaInteractiveCommandSpec{
		ScriptFile:       &prepared.scriptFile,
		WorkDir:          &prepared.workDir,
		ScriptBasePath:   &prepared.scriptBasePath,
		EnvJSON:          LuaInteractiveEnvJSON(prepared.envJSON),
		Args:             LuaInteractiveArgs(append([]string(nil), ctx.PositionalArgs...)),
		AllowedBinaries:  prepared.allowedBinaries,
		BinaryLookupMode: prepared.binaryLookupMode,
		FilesystemAccess: prepared.filesystemAccess,
		FilesystemPaths:  prepared.filesystemPaths,
		EnableUroot:      r.utilitiesEnabled,
	}
	if prepared.runtimeCfg != nil {
		spec.CPULimit = prepared.runtimeCfg.CPULimit
		spec.MemoryLimit = prepared.runtimeCfg.MemoryLimit
	}
	if validateErr := spec.Validate(); validateErr != nil {
		prepared.cleanup()
		return nil, fmt.Errorf("invalid virtual-lua interactive command spec: %w", validateErr)
	}

	cmd, err := r.interactiveCommandFactory(ctx.Context, spec)
	if err != nil {
		prepared.cleanup()
		return nil, fmt.Errorf("create virtual-lua interactive subprocess: %w", err)
	}

	return &PreparedCommand{Cmd: cmd, Cleanup: prepared.cleanup}, nil
}

// RunLuaScript executes a virtual-lua script with the same bridge and safety
// semantics used by LuaRuntime. It is used by the internal CLI subprocess
// wrapper for interactive PTY execution.
//
//nolint:contextcheck // nil context is accepted for internal subprocess compatibility.
func RunLuaScript(ctx context.Context, opts LuaScriptOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := opts.Validate(); err != nil {
		return err
	}
	if interpErr := validateLuaInterpreter(invowkfile.ImplementationScript{}, opts.Script); interpErr != nil {
		return interpErr
	}

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
	runtimeCfg := &invowkfile.RuntimeConfig{
		Name:        invowkfile.RuntimeVirtualLua,
		CPULimit:    opts.CPULimit,
		MemoryLimit: opts.MemoryLimit,
	}
	rt := NewLuaRuntime(opts.EnableUroot)
	result := rt.executeScript(ctx, luaExecutionRequest{
		script:         opts.Script,
		runtimeCfg:     runtimeCfg,
		env:            env,
		policy:         binaryPolicy,
		pathResolver:   pathResolver,
		pathValidator:  pathValidator,
		workDir:        opts.WorkDir,
		scriptBasePath: opts.ScriptBasePath,
		args:           opts.Args,
		stdin:          opts.Stdin,
		stdout:         opts.Stdout,
		stderr:         opts.Stderr,
	})
	if result.Success() {
		return nil
	}
	if result.Error != nil {
		return result.Error
	}
	return fmt.Errorf("lua script exited with code %d", result.ExitCode)
}

// Validate returns nil when direct subprocess execution options are valid.
func (o LuaScriptOptions) Validate() error {
	var errs []error
	if o.WorkDir != "" {
		path := types.FilesystemPath(o.WorkDir) //goplint:ignore -- subprocess CLI boundary path validated immediately below.
		if err := path.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if o.ScriptBasePath != "" {
		path := types.FilesystemPath(o.ScriptBasePath) //goplint:ignore -- subprocess CLI boundary path validated immediately below.
		if err := path.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, raw := range o.AllowedBinaries {
		binary := invowkfile.AllowedBinary(raw) //goplint:ignore -- subprocess CLI boundary value validated immediately below.
		if err := binary.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := o.BinaryLookupMode.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := o.FilesystemAccess.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := o.FilesystemPaths.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := o.CPULimit.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := o.MemoryLimit.Validate(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
