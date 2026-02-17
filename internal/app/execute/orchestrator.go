// SPDX-License-Identifier: MPL-2.0

package execute

import (
	"fmt"
	"strings"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
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
		Runtime     string
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

		EnvInheritMode  string
		EnvInheritAllow []string
		EnvInheritDeny  []string
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
func ResolveRuntime(command *invowkfile.Command, commandName, runtimeOverride string, cfg *config.Config) (RuntimeSelection, error) {
	currentPlatform := invowkfile.GetCurrentHostOS()

	if runtimeOverride != "" {
		overrideRuntime := invowkfile.RuntimeMode(runtimeOverride)
		if !command.IsRuntimeAllowedForPlatform(currentPlatform, overrideRuntime) {
			return RuntimeSelection{}, &RuntimeNotAllowedError{
				CommandName: commandName,
				Runtime:     runtimeOverride,
				Platform:    currentPlatform,
				Allowed:     command.GetAllowedRuntimesForPlatform(currentPlatform),
			}
		}

		impl := command.GetImplForPlatformRuntime(currentPlatform, overrideRuntime)
		if impl == nil {
			return RuntimeSelection{}, fmt.Errorf(
				"no implementation found for command '%s' on platform '%s' with runtime '%s'",
				commandName,
				currentPlatform,
				overrideRuntime,
			)
		}
		return RuntimeSelection{Mode: overrideRuntime, Impl: impl}, nil
	}

	if cfg != nil && cfg.DefaultRuntime != "" {
		configRuntime := invowkfile.RuntimeMode(cfg.DefaultRuntime)
		if command.IsRuntimeAllowedForPlatform(currentPlatform, configRuntime) {
			impl := command.GetImplForPlatformRuntime(currentPlatform, configRuntime)
			if impl != nil {
				return RuntimeSelection{Mode: configRuntime, Impl: impl}, nil
			}
		}
	}

	defaultRuntime := command.GetDefaultRuntimeForPlatform(currentPlatform)
	defaultImpl := command.GetImplForPlatformRuntime(currentPlatform, defaultRuntime)
	if defaultImpl == nil {
		return RuntimeSelection{}, fmt.Errorf(
			"no implementation found for command '%s' on platform '%s' with runtime '%s'",
			commandName,
			currentPlatform,
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
		mode, err := invowkfile.ParseEnvInheritMode(opts.EnvInheritMode)
		if err != nil {
			return err
		}
		execCtx.Env.InheritModeOverride = mode
	}

	for _, name := range opts.EnvInheritAllow {
		if err := invowkfile.ValidateEnvVarName(name); err != nil {
			return fmt.Errorf("env-inherit-allow: %w", err)
		}
	}
	if opts.EnvInheritAllow != nil {
		execCtx.Env.InheritAllowOverride = opts.EnvInheritAllow
	}

	for _, name := range opts.EnvInheritDeny {
		if err := invowkfile.ValidateEnvVarName(name); err != nil {
			return fmt.Errorf("env-inherit-deny: %w", err)
		}
	}
	if opts.EnvInheritDeny != nil {
		execCtx.Env.InheritDenyOverride = opts.EnvInheritDeny
	}

	return nil
}

func projectEnvVars(opts BuildExecutionContextOptions, execCtx *runtime.ExecutionContext) {
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
