// SPDX-License-Identifier: MPL-2.0

// Package invkfiletest provides test helpers for creating invkfile.Command objects.
// This package is separate from testutil to avoid import cycles, since testutil
// is used by pkg/invkmod tests which cannot transitively import pkg/invkfile.
//
// Usage:
//
//	import "invowk-cli/internal/testutil/invkfiletest"
//
//	cmd := invkfiletest.NewTestCommand("hello", invkfiletest.WithScript("echo hello"))
package invkfiletest

import (
	"invowk-cli/pkg/invkfile"
)

type (
	// CommandOption configures a test command.
	// Apply options to customize beyond the minimal defaults.
	CommandOption func(*invkfile.Command)

	// FlagOption configures a test command flag.
	FlagOption func(*invkfile.Flag)

	// ArgOption configures a test command argument.
	ArgOption func(*invkfile.Argument)
)

// NewTestCommand creates a test command with the given name and options.
// By default, creates a command with:
//   - Single implementation with RuntimeNative
//   - Empty script (override with WithScript)
//   - No platform restrictions
//   - No flags or arguments
//
// Usage:
//
//	cmd := invkfiletest.NewTestCommand("hello")
//	cmd := invkfiletest.NewTestCommand("hello", invkfiletest.WithScript("echo hello"))
//	cmd := invkfiletest.NewTestCommand("hello",
//	    invkfiletest.WithScript("echo $NAME"),
//	    invkfiletest.WithRuntime(invkfile.RuntimeVirtual),
//	    invkfiletest.WithEnv("NAME", "world"),
//	)
func NewTestCommand(name string, opts ...CommandOption) *invkfile.Command {
	cmd := &invkfile.Command{
		Name: name,
		Implementations: []invkfile.Implementation{
			{
				Runtimes: []invkfile.RuntimeConfig{
					{Name: invkfile.RuntimeNative},
				},
			},
		},
	}
	for _, opt := range opts {
		opt(cmd)
	}
	return cmd
}

// --- Command Options ---

// WithScript sets the implementation script.
// This sets the script on the first (default) implementation.
func WithScript(script string) CommandOption {
	return func(c *invkfile.Command) {
		if len(c.Implementations) > 0 {
			c.Implementations[0].Script = script
		}
	}
}

// WithDescription sets the command description.
func WithDescription(desc string) CommandOption {
	return func(c *invkfile.Command) {
		c.Description = desc
	}
}

// WithRuntime sets a single runtime on the first implementation.
func WithRuntime(r invkfile.RuntimeMode) CommandOption {
	return func(c *invkfile.Command) {
		if len(c.Implementations) > 0 {
			c.Implementations[0].Runtimes = []invkfile.RuntimeConfig{
				{Name: r},
			}
		}
	}
}

// WithRuntimes sets multiple runtimes on the first implementation.
func WithRuntimes(rs ...invkfile.RuntimeMode) CommandOption {
	return func(c *invkfile.Command) {
		if len(c.Implementations) > 0 {
			configs := make([]invkfile.RuntimeConfig, len(rs))
			for i, r := range rs {
				configs[i] = invkfile.RuntimeConfig{Name: r}
			}
			c.Implementations[0].Runtimes = configs
		}
	}
}

// WithRuntimeConfig sets a runtime config on the first implementation.
// Use this for container runtimes that need additional configuration.
func WithRuntimeConfig(rc invkfile.RuntimeConfig) CommandOption {
	return func(c *invkfile.Command) {
		if len(c.Implementations) > 0 {
			c.Implementations[0].Runtimes = []invkfile.RuntimeConfig{rc}
		}
	}
}

// WithPlatform adds a platform constraint to the first implementation.
func WithPlatform(p invkfile.Platform) CommandOption {
	return func(c *invkfile.Command) {
		if len(c.Implementations) > 0 {
			c.Implementations[0].Platforms = append(c.Implementations[0].Platforms,
				invkfile.PlatformConfig{Name: p})
		}
	}
}

// WithPlatforms sets multiple platform constraints on the first implementation.
func WithPlatforms(ps ...invkfile.Platform) CommandOption {
	return func(c *invkfile.Command) {
		if len(c.Implementations) > 0 {
			configs := make([]invkfile.PlatformConfig, len(ps))
			for i, p := range ps {
				configs[i] = invkfile.PlatformConfig{Name: p}
			}
			c.Implementations[0].Platforms = configs
		}
	}
}

// WithEnv adds an environment variable to the command.
func WithEnv(key, value string) CommandOption {
	return func(c *invkfile.Command) {
		if c.Env == nil {
			c.Env = &invkfile.EnvConfig{
				Vars: make(map[string]string),
			}
		}
		if c.Env.Vars == nil {
			c.Env.Vars = make(map[string]string)
		}
		c.Env.Vars[key] = value
	}
}

// WithWorkDir sets the working directory.
func WithWorkDir(dir string) CommandOption {
	return func(c *invkfile.Command) {
		c.WorkDir = dir
	}
}

// WithImplementation adds a custom implementation.
// Use when you need multiple implementations or non-default configuration.
func WithImplementation(impl invkfile.Implementation) CommandOption {
	return func(c *invkfile.Command) {
		c.Implementations = append(c.Implementations, impl)
	}
}

// WithImplementations replaces all implementations.
func WithImplementations(impls ...invkfile.Implementation) CommandOption {
	return func(c *invkfile.Command) {
		c.Implementations = impls
	}
}

// WithDependsOn sets the command's dependencies.
func WithDependsOn(deps *invkfile.DependsOn) CommandOption {
	return func(c *invkfile.Command) {
		c.DependsOn = deps
	}
}

// --- Flag Options ---

// WithFlag adds a flag to the command.
func WithFlag(name string, opts ...FlagOption) CommandOption {
	return func(c *invkfile.Command) {
		f := invkfile.Flag{Name: name}
		for _, opt := range opts {
			opt(&f)
		}
		c.Flags = append(c.Flags, f)
	}
}

// FlagRequired marks the flag as required.
func FlagRequired() FlagOption {
	return func(f *invkfile.Flag) {
		f.Required = true
	}
}

// FlagDefault sets the flag's default value.
func FlagDefault(v string) FlagOption {
	return func(f *invkfile.Flag) {
		f.DefaultValue = v
	}
}

// FlagShorthand sets the flag's single-character shorthand.
func FlagShorthand(s string) FlagOption {
	return func(f *invkfile.Flag) {
		f.Short = s
	}
}

// FlagDescription sets the flag's description.
func FlagDescription(desc string) FlagOption {
	return func(f *invkfile.Flag) {
		f.Description = desc
	}
}

// FlagType sets the flag's type.
func FlagType(t invkfile.FlagType) FlagOption {
	return func(f *invkfile.Flag) {
		f.Type = t
	}
}

// FlagValidation sets the flag's validation regex pattern.
func FlagValidation(pattern string) FlagOption {
	return func(f *invkfile.Flag) {
		f.Validation = pattern
	}
}

// --- Arg Options ---

// WithArg adds a positional argument to the command.
func WithArg(name string, opts ...ArgOption) CommandOption {
	return func(c *invkfile.Command) {
		a := invkfile.Argument{Name: name}
		for _, opt := range opts {
			opt(&a)
		}
		c.Args = append(c.Args, a)
	}
}

// ArgRequired marks the argument as required.
func ArgRequired() ArgOption {
	return func(a *invkfile.Argument) {
		a.Required = true
	}
}

// ArgDefault sets the argument's default value.
func ArgDefault(v string) ArgOption {
	return func(a *invkfile.Argument) {
		a.DefaultValue = v
	}
}

// ArgVariadic marks the argument as variadic (accepts multiple values).
func ArgVariadic() ArgOption {
	return func(a *invkfile.Argument) {
		a.Variadic = true
	}
}

// ArgDescription sets the argument's description.
func ArgDescription(desc string) ArgOption {
	return func(a *invkfile.Argument) {
		a.Description = desc
	}
}

// ArgType sets the argument's type.
func ArgType(t invkfile.ArgumentType) ArgOption {
	return func(a *invkfile.Argument) {
		a.Type = t
	}
}

// ArgValidation sets the argument's validation regex pattern.
func ArgValidation(pattern string) ArgOption {
	return func(a *invkfile.Argument) {
		a.Validation = pattern
	}
}
