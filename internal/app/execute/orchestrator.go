// SPDX-License-Identifier: MPL-2.0

package execute

import (
	"fmt"
	"strings"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
	platmeta "github.com/invowk/invowk/pkg/platform"
)

type (
	// RuntimeSelection is the resolved runtime mode + implementation pair.
	RuntimeSelection struct {
		Mode invowkfile.RuntimeMode
		Impl *invowkfile.Implementation
	}

	// RuntimeNotAllowedError indicates a runtime override incompatible with the command.
	RuntimeNotAllowedError struct {
		CommandName string
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
		Workdir      string
		ForceRebuild bool

		EnvFiles []string
		EnvVars  map[string]string

		FlagValues map[string]string
		ArgDefs    []invowkfile.Argument

		EnvInheritMode  invowkfile.EnvInheritMode
		EnvInheritAllow []string
		EnvInheritDeny  []string

		// SourceID identifies the origin of the command (invowkfile path or module ID).
		// Injected as INVOWK_SOURCE so scripts can identify which source they belong to.
		SourceID string
		// Platform is the resolved platform for this execution.
		// Injected as INVOWK_PLATFORM so scripts can self-introspect the target platform.
		Platform invowkfile.Platform
	}
)

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
func ResolveRuntime(command *invowkfile.Command, commandName string, runtimeOverride invowkfile.RuntimeMode, cfg *config.Config, platform invowkfile.Platform) (RuntimeSelection, error) {
	if runtimeOverride != "" {
		// Defense-in-depth: the CLI boundary should have already validated the mode
		// via ParseRuntimeMode, but verify here to catch programmatic misuse.
		if !runtimeOverride.IsValid() {
			return RuntimeSelection{}, fmt.Errorf("invalid runtime override %q (expected: native, virtual, container)", runtimeOverride)
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
		return RuntimeSelection{Mode: runtimeOverride, Impl: impl}, nil
	}

	if cfg != nil && cfg.DefaultRuntime != "" {
		configRuntime := invowkfile.RuntimeMode(cfg.DefaultRuntime)
		// Defense-in-depth: CUE schema validates config at load time, but verify
		// here to prevent silent fallthrough to command default on invalid config.
		if !configRuntime.IsValid() {
			return RuntimeSelection{}, fmt.Errorf(
				"invalid default_runtime %q in config (expected: native, virtual, container)",
				cfg.DefaultRuntime,
			)
		}
		if command.IsRuntimeAllowedForPlatform(platform, configRuntime) {
			impl := command.GetImplForPlatformRuntime(platform, configRuntime)
			if impl != nil {
				return RuntimeSelection{Mode: configRuntime, Impl: impl}, nil
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

	return RuntimeSelection{Mode: defaultRuntime, Impl: defaultImpl}, nil
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

	execCtx := runtime.NewExecutionContext(opts.Command, opts.Invowkfile)

	execCtx.Verbose = opts.Verbose
	execCtx.SelectedRuntime = opts.Selection.Mode
	execCtx.SelectedImpl = opts.Selection.Impl
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
		if !opts.EnvInheritMode.IsValid() {
			return fmt.Errorf("invalid env_inherit_mode %q (expected: none, allow, all)", opts.EnvInheritMode)
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

func validateEnvVarNames(names []string, label string) error {
	for _, name := range names {
		if err := invowkfile.ValidateEnvVarName(name); err != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
	}
	return nil
}

func projectEnvVars(opts BuildExecutionContextOptions, execCtx *runtime.ExecutionContext) {
	// Metadata env vars for script self-introspection.
	// These allow scripts to know which command, runtime, source, and platform they run under.
	execCtx.Env.ExtraEnv[platmeta.EnvVarCmdName] = opts.Command.Name
	execCtx.Env.ExtraEnv[platmeta.EnvVarRuntime] = string(opts.Selection.Mode)
	// EnvVarSource and EnvVarPlatform are conditionally injected (only when
	// non-empty), but unconditionally filtered in shouldFilterEnvVar. The
	// asymmetry is intentional: filtering prevents leakage even if future
	// code paths inject these vars unconditionally.
	if opts.SourceID != "" {
		execCtx.Env.ExtraEnv[platmeta.EnvVarSource] = opts.SourceID
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
