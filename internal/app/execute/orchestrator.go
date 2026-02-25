// SPDX-License-Identifier: MPL-2.0

package execute

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
	platmeta "github.com/invowk/invowk/pkg/platform"
)

// ErrInvalidRuntimeSelection is the sentinel error wrapped by InvalidRuntimeSelectionError.
var ErrInvalidRuntimeSelection = errors.New("invalid runtime selection")

type (
	// RuntimeSelection is the resolved runtime mode + implementation pair.
	// Fields are unexported for immutability; use Mode() and Impl() accessors.
	RuntimeSelection struct {
		mode invowkfile.RuntimeMode
		impl *invowkfile.Implementation
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

	// BuildExecutionContextOptions configures execution-context construction.
	//
	// Required fields: Command, Invowkfile, and Selection must be populated
	// (Command and Invowkfile must be non-nil; Selection.Impl must be non-nil).
	// All other fields are optional and default to their zero values.
	BuildExecutionContextOptions struct {
		Command    *invowkfile.Command
		Invowkfile *invowkfile.Invowkfile
		Selection  RuntimeSelection

		Args         []string
		Verbose      bool
		Workdir      invowkfile.WorkDir
		ForceRebuild bool

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
// Mode must be a valid RuntimeMode and Impl must not be nil.
func NewRuntimeSelection(mode invowkfile.RuntimeMode, impl *invowkfile.Implementation) (RuntimeSelection, error) {
	if impl == nil {
		return RuntimeSelection{}, fmt.Errorf("implementation must not be nil for runtime mode %q", mode)
	}
	if isValid, errs := mode.IsValid(); !isValid {
		return RuntimeSelection{}, errs[0]
	}
	return RuntimeSelection{mode: mode, impl: impl}, nil
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

// Impl returns the resolved implementation.
func (r RuntimeSelection) Impl() *invowkfile.Implementation { return r.impl }

// IsValid returns whether the RuntimeSelection has valid fields.
// Mode must be a recognized RuntimeMode and Impl must not be nil.
// A selection created via NewRuntimeSelection always passes IsValid();
// selections from RuntimeSelectionOf (test fixtures) may not.
func (r RuntimeSelection) IsValid() (bool, []error) {
	var errs []error
	if valid, fieldErrs := r.mode.IsValid(); !valid {
		errs = append(errs, fieldErrs...)
	}
	if r.impl == nil {
		errs = append(errs, fmt.Errorf("implementation must not be nil"))
	}
	if len(errs) > 0 {
		return false, []error{&InvalidRuntimeSelectionError{FieldErrors: errs}}
	}
	return true, nil
}

// Error implements the error interface for InvalidRuntimeSelectionError.
func (e *InvalidRuntimeSelectionError) Error() string {
	return fmt.Sprintf("invalid runtime selection: %d field error(s)", len(e.FieldErrors))
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
		if isValid, errs := runtimeOverride.IsValid(); !isValid {
			return RuntimeSelection{}, errs[0]
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
		return NewRuntimeSelection(runtimeOverride, impl)
	}

	if cfg != nil && cfg.DefaultRuntime != "" {
		configRuntime := invowkfile.RuntimeMode(cfg.DefaultRuntime)
		// Defense-in-depth: CUE schema validates config at load time, but verify
		// here to prevent silent fallthrough to command default on invalid config.
		if isValid, errs := configRuntime.IsValid(); !isValid {
			return RuntimeSelection{}, fmt.Errorf("invalid default_runtime in config: %w", errs[0])
		}
		if command.IsRuntimeAllowedForPlatform(platform, configRuntime) {
			impl := command.GetImplForPlatformRuntime(platform, configRuntime)
			if impl != nil {
				// Mode is validated above; constructor re-validates (defense-in-depth).
				return NewRuntimeSelection(configRuntime, impl)
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

	return NewRuntimeSelection(defaultRuntime, defaultImpl)
}

// BuildExecutionContext converts options into a runtime.ExecutionContext.
// It validates env inheritance overrides (mode, allow, deny) and returns an error
// for invalid values. Flags and arguments are projected into INVOWK_FLAG_*,
// INVOWK_ARG_*, ARGn, and ARGC environment variables.
func BuildExecutionContext(opts BuildExecutionContextOptions) (*runtime.ExecutionContext, error) {
	if opts.Command == nil {
		return nil, fmt.Errorf("BuildExecutionContext: Command must not be nil")
	}
	if opts.Invowkfile == nil {
		return nil, fmt.Errorf("BuildExecutionContext: Invowkfile must not be nil")
	}

	execCtx := runtime.NewExecutionContext(context.Background(), opts.Command, opts.Invowkfile)

	execCtx.Verbose = opts.Verbose
	execCtx.SelectedRuntime = opts.Selection.Mode()
	execCtx.SelectedImpl = opts.Selection.Impl()
	execCtx.PositionalArgs = opts.Args
	execCtx.WorkDir = opts.Workdir
	execCtx.ForceRebuild = opts.ForceRebuild
	execCtx.Env.RuntimeEnvFiles = opts.EnvFiles
	execCtx.Env.RuntimeEnvVars = opts.EnvVars

	if err := applyEnvInheritOverrides(opts, execCtx); err != nil {
		return nil, err
	}

	projectEnvVars(opts, execCtx)
	return execCtx, nil
}

func applyEnvInheritOverrides(opts BuildExecutionContextOptions, execCtx *runtime.ExecutionContext) error {
	if opts.EnvInheritMode != "" {
		// Defense-in-depth: the CLI boundary should have already validated the mode
		// via ParseEnvInheritMode, but verify here to catch programmatic misuse.
		if isValid, errs := opts.EnvInheritMode.IsValid(); !isValid {
			return errs[0]
		}
		execCtx.Env.InheritModeOverride = opts.EnvInheritMode
	}

	if err := validateEnvVarNames(opts.EnvInheritAllow, "env-inherit-allow"); err != nil {
		return err
	}
	if opts.EnvInheritAllow != nil {
		execCtx.Env.InheritAllowOverride = opts.EnvInheritAllow
	}

	if err := validateEnvVarNames(opts.EnvInheritDeny, "env-inherit-deny"); err != nil {
		return err
	}
	if opts.EnvInheritDeny != nil {
		execCtx.Env.InheritDenyOverride = opts.EnvInheritDeny
	}

	return nil
}

func validateEnvVarNames(names []invowkfile.EnvVarName, label string) error {
	for _, name := range names {
		if isValid, errs := name.IsValid(); !isValid {
			return fmt.Errorf("%s: %w", label, errs[0])
		}
	}
	return nil
}

func projectEnvVars(opts BuildExecutionContextOptions, execCtx *runtime.ExecutionContext) {
	// Metadata env vars for script self-introspection.
	// These allow scripts to know which command, runtime, source, and platform they run under.
	execCtx.Env.ExtraEnv[platmeta.EnvVarCmdName] = string(opts.Command.Name)
	execCtx.Env.ExtraEnv[platmeta.EnvVarRuntime] = string(opts.Selection.Mode())
	// EnvVarSource and EnvVarPlatform are conditionally injected (only when
	// non-empty), but unconditionally filtered in shouldFilterEnvVar. The
	// asymmetry is intentional: filtering prevents leakage even if future
	// code paths inject these vars unconditionally.
	if opts.SourceID != "" {
		execCtx.Env.ExtraEnv[platmeta.EnvVarSource] = string(opts.SourceID)
	}
	if opts.Platform != "" {
		execCtx.Env.ExtraEnv[platmeta.EnvVarPlatform] = string(opts.Platform)
	}

	for i, arg := range opts.Args {
		execCtx.Env.ExtraEnv[fmt.Sprintf("ARG%d", i+1)] = arg
	}
	execCtx.Env.ExtraEnv["ARGC"] = fmt.Sprintf("%d", len(opts.Args))

	if len(opts.ArgDefs) > 0 {
		for i, argDef := range opts.ArgDefs {
			envName := invowkfile.ArgNameToEnvVar(argDef.Name)

			switch {
			case argDef.Variadic:
				var variadicValues []string
				if i < len(opts.Args) {
					variadicValues = opts.Args[i:]
				}
				execCtx.Env.ExtraEnv[envName+"_COUNT"] = fmt.Sprintf("%d", len(variadicValues))
				for j, val := range variadicValues {
					execCtx.Env.ExtraEnv[fmt.Sprintf("%s_%d", envName, j+1)] = val
				}
				execCtx.Env.ExtraEnv[envName] = strings.Join(variadicValues, " ")
			case i < len(opts.Args):
				execCtx.Env.ExtraEnv[envName] = opts.Args[i]
			case argDef.DefaultValue != "":
				execCtx.Env.ExtraEnv[envName] = argDef.DefaultValue
			}
		}
	}

	for name, value := range opts.FlagValues {
		execCtx.Env.ExtraEnv[invowkfile.FlagNameToEnvVar(name)] = value
	}
}
