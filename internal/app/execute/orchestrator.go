// SPDX-License-Identifier: MPL-2.0

package execute

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

// ErrInvalidRuntimeSelection is the sentinel error wrapped by InvalidRuntimeSelectionError.
var ErrInvalidRuntimeSelection = errors.New("invalid runtime selection")

type (
	// RuntimeSelection is the resolved runtime mode + implementation pair.
	// Fields are unexported for immutability; use Mode() and Impl() accessors.
	RuntimeSelection struct {
		mode     invowkfile.RuntimeMode
		platform invowkfile.Platform
		impl     *invowkfile.Implementation
	}

	// InvalidRuntimeSelectionError is returned when a RuntimeSelection has invalid fields.
	// It wraps ErrInvalidRuntimeSelection for errors.Is() compatibility and collects
	// field-level validation errors from Mode and Impl.
	InvalidRuntimeSelectionError struct {
		FieldErrors []error
	}

	// RuntimeNotAllowedError indicates a runtime override incompatible with the command.
	RuntimeNotAllowedError struct {
		CommandName invowkfile.CommandName
		Runtime     invowkfile.RuntimeMode
		Platform    invowkfile.Platform
		Allowed     []invowkfile.RuntimeMode
	}

	//goplint:validate-all
	//
	// BuildExecutionContextOptions configures execution-context construction.
	//
	// Required fields: Command, Invowkfile, and Selection must be populated
	// (Command and Invowkfile must be non-nil; Selection.Impl must be non-nil).
	// All other fields are optional and default to their zero values.
	BuildExecutionContextOptions struct {
		Command         *invowkfile.Command
		CommandFullName invowkfile.CommandName //goplint:ignore -- optional discovery metadata validated when non-empty.
		Invowkfile      *invowkfile.Invowkfile
		Selection       RuntimeSelection

		Args          []string
		Verbose       bool
		Workdir       invowkfile.WorkDir
		ForceRebuild  bool
		ContainerName invowkfile.ContainerName

		EnvFiles []invowkfile.DotenvFilePath
		EnvVars  map[string]string

		FlagValues map[invowkfile.FlagName]string
		ArgDefs    []invowkfile.Argument

		EnvInheritMode  invowkfile.EnvInheritMode
		EnvInheritAllow []invowkfile.EnvVarName
		EnvInheritDeny  []invowkfile.EnvVarName

		// SourceID identifies the origin of the command (invowkfile path or module ID).
		// Injected as INVOWK_SOURCE so scripts can identify which source they belong to.
		SourceID discovery.SourceID
		// Platform is the resolved platform for this execution.
		// Injected as INVOWK_PLATFORM so scripts can self-introspect the target platform.
		Platform invowkfile.Platform
	}
)

// NewRuntimeSelection creates a validated RuntimeSelection.
// Mode must be a valid RuntimeMode, platform must be a valid Platform when
// provided, and Impl must not be nil.
func NewRuntimeSelection(mode invowkfile.RuntimeMode, platform invowkfile.Platform, impl *invowkfile.Implementation) (RuntimeSelection, error) {
	if impl == nil {
		return RuntimeSelection{}, fmt.Errorf("implementation must not be nil for runtime mode %q", mode)
	}
	if err := mode.Validate(); err != nil {
		return RuntimeSelection{}, err
	}
	if platform != "" {
		if err := platform.Validate(); err != nil {
			return RuntimeSelection{}, err
		}
	}
	return RuntimeSelection{mode: mode, platform: platform, impl: impl}, nil
}

// RuntimeSelectionOf creates a RuntimeSelection without validation.
// Prefer NewRuntimeSelection in production code. This variant is for test
// fixtures and rendering paths where an incomplete selection is valid
// (e.g., nil Impl for dry-run rendering edge cases).
func RuntimeSelectionOf(mode invowkfile.RuntimeMode, impl *invowkfile.Implementation) RuntimeSelection {
	return RuntimeSelection{mode: mode, impl: impl}
}

// Mode returns the resolved runtime mode.
func (r RuntimeSelection) Mode() invowkfile.RuntimeMode { return r.mode }

// Platform returns the platform used to select the implementation.
func (r RuntimeSelection) Platform() invowkfile.Platform { return r.platform }

// Impl returns the resolved implementation.
func (r RuntimeSelection) Impl() *invowkfile.Implementation { return r.impl }

// Validate returns nil if the RuntimeSelection has valid fields, or an error if not.
// Mode must be a recognized RuntimeMode and Impl must not be nil.
// A selection created via NewRuntimeSelection always passes Validate();
// selections from RuntimeSelectionOf (test fixtures) may not.
func (r RuntimeSelection) Validate() error {
	var errs []error
	if err := r.mode.Validate(); err != nil {
		errs = append(errs, err)
	}
	if r.platform != "" {
		if err := r.platform.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if r.impl == nil {
		errs = append(errs, errors.New("implementation must not be nil"))
	}
	if len(errs) > 0 {
		return &InvalidRuntimeSelectionError{FieldErrors: errs}
	}
	return nil
}

// Error implements the error interface for InvalidRuntimeSelectionError.
func (e *InvalidRuntimeSelectionError) Error() string {
	return types.FormatFieldErrors("runtime selection", e.FieldErrors)
}

// Unwrap returns ErrInvalidRuntimeSelection for errors.Is() compatibility.
func (e *InvalidRuntimeSelectionError) Unwrap() error { return ErrInvalidRuntimeSelection }

func (e *RuntimeNotAllowedError) Error() string {
	allowed := make([]string, len(e.Allowed))
	for i, r := range e.Allowed {
		allowed[i] = string(r)
	}
	return fmt.Sprintf(
		"runtime '%s' is not allowed for command '%s' on platform '%s' (allowed: %s)",
		e.Runtime,
		e.CommandName,
		e.Platform,
		strings.Join(allowed, ", "),
	)
}

// ResolveRuntime applies runtime-selection precedence:
//  1. CLI override (hard fail if incompatible)
//  2. Config default runtime (soft fallback)
//  3. Command default runtime
//
// The platform parameter makes this function pure — callers pass the resolved
// platform rather than relying on the host OS at call time. Production code
// passes invowkfile.CurrentPlatform(); tests pass a fixed platform for
// deterministic behavior across CI environments.
func ResolveRuntime(command *invowkfile.Command, commandName invowkfile.CommandName, runtimeOverride invowkfile.RuntimeMode, cfg *config.Config, platform invowkfile.Platform) (RuntimeSelection, error) {
	if runtimeOverride != "" {
		// Defense-in-depth: the CLI boundary should have already validated the mode
		// via ParseRuntimeMode, but verify here to catch programmatic misuse.
		if err := runtimeOverride.Validate(); err != nil {
			return RuntimeSelection{}, err
		}

		if !command.IsRuntimeAllowedForPlatform(platform, runtimeOverride) {
			return RuntimeSelection{}, &RuntimeNotAllowedError{
				CommandName: commandName,
				Runtime:     runtimeOverride,
				Platform:    platform,
				Allowed:     command.GetAllowedRuntimesForPlatform(platform),
			}
		}

		impl := command.GetImplForPlatformRuntime(platform, runtimeOverride)
		if impl == nil {
			return RuntimeSelection{}, fmt.Errorf(
				"no implementation found for command '%s' on platform '%s' with runtime '%s'",
				commandName,
				platform,
				runtimeOverride,
			)
		}
		// Mode is validated above; constructor re-validates (defense-in-depth).
		return NewRuntimeSelection(runtimeOverride, platform, impl)
	}

	if cfg != nil && cfg.DefaultRuntime != "" {
		configRuntime := cfg.DefaultRuntime
		// Defense-in-depth: CUE schema validates config at load time, but verify
		// here to prevent silent fallthrough to command default on invalid config.
		if err := configRuntime.Validate(); err != nil {
			return RuntimeSelection{}, fmt.Errorf("invalid default_runtime in config: %w", err)
		}
		if command.IsRuntimeAllowedForPlatform(platform, configRuntime) {
			impl := command.GetImplForPlatformRuntime(platform, configRuntime)
			if impl != nil {
				// Mode is validated above; constructor re-validates (defense-in-depth).
				return NewRuntimeSelection(configRuntime, platform, impl)
			}
		}
	}

	defaultRuntime := command.GetDefaultRuntimeForPlatform(platform)
	defaultImpl := command.GetImplForPlatformRuntime(platform, defaultRuntime)
	if defaultImpl == nil {
		return RuntimeSelection{}, fmt.Errorf(
			"no implementation found for command '%s' on platform '%s' with runtime '%s'",
			commandName,
			platform,
			defaultRuntime,
		)
	}

	return NewRuntimeSelection(defaultRuntime, platform, defaultImpl)
}

// BuildExecutionContext converts options into a runtime.ExecutionContext.
// The ctx parameter controls cancellation and timeout propagation; when nil,
// it defaults to context.Background().
// It validates env inheritance overrides (mode, allow, deny) and returns an error
// for invalid values. Flags and arguments are projected into INVOWK_FLAG_*,
// INVOWK_ARG_*, ARGn, and ARGC environment variables.
//
//nolint:contextcheck // nil context is an explicit boundary fallback documented above.
func BuildExecutionContext(ctx context.Context, opts BuildExecutionContextOptions) (*runtime.ExecutionContext, error) {
	if opts.Command == nil {
		return nil, errors.New("BuildExecutionContext: Command must not be nil")
	}
	if opts.Invowkfile == nil {
		return nil, errors.New("BuildExecutionContext: Invowkfile must not be nil")
	}

	// Validate typed fields (Selection, Workdir, EnvFiles, EnvInheritMode, etc.)
	// after the nil pointer guards above to produce clear error messages.
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid execution context options: %w", err)
	}

	if ctx == nil {
		ctx = context.Background()
	}
	execCtx := runtime.NewExecutionContext(ctx, opts.Command, opts.Invowkfile)
	selectedPlatform := opts.Selection.Platform()
	if opts.Platform != "" {
		selectedPlatform = opts.Platform
	}

	execCtx.Verbose = opts.Verbose
	execCtx.SelectedRuntime = opts.Selection.Mode()
	if selectedPlatform != "" {
		execCtx.SelectedPlatform = selectedPlatform
	}
	execCtx.SelectedImpl = opts.Selection.Impl()
	execCtx.PositionalArgs = opts.Args
	execCtx.WorkDir = opts.Workdir
	execCtx.ForceRebuild = opts.ForceRebuild
	execCtx.ContainerNameOverride = opts.ContainerName
	execCtx.CommandFullName = opts.CommandFullName
	execCtx.Env.RuntimeEnvFiles = opts.EnvFiles
	execCtx.Env.RuntimeEnvVars = opts.EnvVars

	applyEnvInheritOverrides(opts, execCtx)

	projectEnvVars(opts, execCtx, selectedPlatform)
	return execCtx, nil
}

func applyEnvInheritOverrides(opts BuildExecutionContextOptions, execCtx *runtime.ExecutionContext) {
	if opts.EnvInheritMode != "" {
		execCtx.Env.InheritModeOverride = opts.EnvInheritMode
	}

	if opts.EnvInheritAllow != nil {
		execCtx.Env.InheritAllowOverride = opts.EnvInheritAllow
	}

	if opts.EnvInheritDeny != nil {
		execCtx.Env.InheritDenyOverride = opts.EnvInheritDeny
	}
}

func projectEnvVars(opts BuildExecutionContextOptions, execCtx *runtime.ExecutionContext, selectedPlatform invowkfile.Platform) {
	projectCommandEnvVars(opts, execCtx, selectedPlatform)
	projectArgEnvVars(opts, execCtx)
	projectFlagEnvVars(opts, execCtx)
}

func projectCommandEnvVars(opts BuildExecutionContextOptions, execCtx *runtime.ExecutionContext, selectedPlatform invowkfile.Platform) {
	execCtx.Env.ExtraEnv[runtime.EnvVarCmdName] = string(opts.Command.Name)
	execCtx.Env.ExtraEnv[runtime.EnvVarRuntime] = string(opts.Selection.Mode())
	// EnvVarSource and EnvVarPlatform are conditionally injected (only when
	// non-empty), but unconditionally filtered in shouldFilterEnvVar. The
	// asymmetry is intentional: filtering prevents leakage even if future
	// code paths inject these vars unconditionally.
	if opts.SourceID != "" {
		execCtx.Env.ExtraEnv[runtime.EnvVarSource] = string(opts.SourceID)
	}
	if selectedPlatform != "" {
		execCtx.Env.ExtraEnv[runtime.EnvVarPlatform] = string(selectedPlatform)
	}
}

func projectArgEnvVars(opts BuildExecutionContextOptions, execCtx *runtime.ExecutionContext) {
	for i, arg := range opts.Args {
		execCtx.Env.ExtraEnv[fmt.Sprintf("ARG%d", i+1)] = arg
	}
	execCtx.Env.ExtraEnv["ARGC"] = strconv.Itoa(len(opts.Args))

	for i, argDef := range opts.ArgDefs {
		projectArgDefEnvVar(opts, execCtx, i, argDef)
	}
}

//goplint:ignore -- argument index maps user argv positions into conventional ARG-style environment variables.
func projectArgDefEnvVar(opts BuildExecutionContextOptions, execCtx *runtime.ExecutionContext, index int, argDef invowkfile.Argument) {
	envName := invowkfile.ArgNameToEnvVar(argDef.Name)
	switch {
	case argDef.Variadic:
		projectVariadicArgEnvVars(opts, execCtx, index, envName)
	case index < len(opts.Args):
		execCtx.Env.ExtraEnv[envName] = opts.Args[index]
	case argDef.DefaultValue != "":
		execCtx.Env.ExtraEnv[envName] = argDef.DefaultValue
	}
}

//goplint:ignore -- argument index and envName are derived adapter values for exported process environment keys.
func projectVariadicArgEnvVars(opts BuildExecutionContextOptions, execCtx *runtime.ExecutionContext, index int, envName string) {
	values := opts.Args[min(index, len(opts.Args)):]
	execCtx.Env.ExtraEnv[envName+"_COUNT"] = strconv.Itoa(len(values))
	for i, value := range values {
		execCtx.Env.ExtraEnv[fmt.Sprintf("%s_%d", envName, i+1)] = value
	}
	execCtx.Env.ExtraEnv[envName] = strings.Join(values, " ")
}

func projectFlagEnvVars(opts BuildExecutionContextOptions, execCtx *runtime.ExecutionContext) {
	for name, value := range opts.FlagValues {
		execCtx.Env.ExtraEnv[invowkfile.FlagNameToEnvVar(name)] = value
	}
}
