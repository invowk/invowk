// SPDX-License-Identifier: MPL-2.0

package invowkfiletest

import (
	"github.com/invowk/invowk/pkg/invowkfile"
)

type (
	// CommandOption configures a test command.
	// Apply options to customize beyond the minimal defaults.
	CommandOption func(*invowkfile.Command)

	// FlagOption configures a test command flag.
	FlagOption func(*invowkfile.Flag)

	// ArgOption configures a test command argument.
	ArgOption func(*invowkfile.Argument)
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
//	cmd := invowkfiletest.NewTestCommand("hello")
//	cmd := invowkfiletest.NewTestCommand("hello", invowkfiletest.WithScript("echo hello"))
//	cmd := invowkfiletest.NewTestCommand("hello",
//	    invowkfiletest.WithScript("echo $NAME"),
//	    invowkfiletest.WithRuntime(invowkfile.RuntimeVirtual),
//	    invowkfiletest.WithEnv("NAME", "world"),
//	)
func NewTestCommand(name string, opts ...CommandOption) *invowkfile.Command {
	cmd := &invowkfile.Command{
		Name: invowkfile.CommandName(name),
		Implementations: []invowkfile.Implementation{
			{
				Runtimes: []invowkfile.RuntimeConfig{
					{Name: invowkfile.RuntimeNative},
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
	return func(c *invowkfile.Command) {
		if len(c.Implementations) > 0 {
			c.Implementations[0].Script = invowkfile.ScriptContent(script)
		}
	}
}

// WithDescription sets the command description.
func WithDescription(desc string) CommandOption {
	return func(c *invowkfile.Command) {
		c.Description = desc
	}
}

// WithRuntime sets a single runtime on the first implementation.
func WithRuntime(r invowkfile.RuntimeMode) CommandOption {
	return func(c *invowkfile.Command) {
		if len(c.Implementations) > 0 {
			c.Implementations[0].Runtimes = []invowkfile.RuntimeConfig{
				{Name: r},
			}
		}
	}
}

// WithRuntimes sets multiple runtimes on the first implementation.
func WithRuntimes(rs ...invowkfile.RuntimeMode) CommandOption {
	return func(c *invowkfile.Command) {
		if len(c.Implementations) > 0 {
			configs := make([]invowkfile.RuntimeConfig, len(rs))
			for i, r := range rs {
				configs[i] = invowkfile.RuntimeConfig{Name: r}
			}
			c.Implementations[0].Runtimes = configs
		}
	}
}

// WithRuntimeConfig sets a runtime config on the first implementation.
// Use this for container runtimes that need additional configuration.
func WithRuntimeConfig(rc invowkfile.RuntimeConfig) CommandOption {
	return func(c *invowkfile.Command) {
		if len(c.Implementations) > 0 {
			c.Implementations[0].Runtimes = []invowkfile.RuntimeConfig{rc}
		}
	}
}

// WithPlatform adds a platform constraint to the first implementation.
func WithPlatform(p invowkfile.Platform) CommandOption {
	return func(c *invowkfile.Command) {
		if len(c.Implementations) > 0 {
			c.Implementations[0].Platforms = append(c.Implementations[0].Platforms,
				invowkfile.PlatformConfig{Name: p})
		}
	}
}

// WithPlatforms sets multiple platform constraints on the first implementation.
func WithPlatforms(ps ...invowkfile.Platform) CommandOption {
	return func(c *invowkfile.Command) {
		if len(c.Implementations) > 0 {
			configs := make([]invowkfile.PlatformConfig, len(ps))
			for i, p := range ps {
				configs[i] = invowkfile.PlatformConfig{Name: p}
			}
			c.Implementations[0].Platforms = configs
		}
	}
}

// WithEnv adds an environment variable to the command.
func WithEnv(key, value string) CommandOption {
	return func(c *invowkfile.Command) {
		if c.Env == nil {
			c.Env = &invowkfile.EnvConfig{
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
	return func(c *invowkfile.Command) {
		c.WorkDir = invowkfile.WorkDir(dir)
	}
}

// WithImplementation adds a custom implementation.
// Use when you need multiple implementations or non-default configuration.
func WithImplementation(impl invowkfile.Implementation) CommandOption {
	return func(c *invowkfile.Command) {
		c.Implementations = append(c.Implementations, impl)
	}
}

// WithImplementations replaces all implementations.
func WithImplementations(impls ...invowkfile.Implementation) CommandOption {
	return func(c *invowkfile.Command) {
		c.Implementations = impls
	}
}

// WithDependsOn sets the command's dependencies.
func WithDependsOn(deps *invowkfile.DependsOn) CommandOption {
	return func(c *invowkfile.Command) {
		c.DependsOn = deps
	}
}

// --- Flag Options ---

// WithFlag adds a flag to the command.
func WithFlag(name string, opts ...FlagOption) CommandOption {
	return func(c *invowkfile.Command) {
		f := invowkfile.Flag{Name: invowkfile.FlagName(name)}
		for _, opt := range opts {
			opt(&f)
		}
		c.Flags = append(c.Flags, f)
	}
}

// FlagRequired marks the flag as required.
func FlagRequired() FlagOption {
	return func(f *invowkfile.Flag) {
		f.Required = true
	}
}

// FlagDefault sets the flag's default value.
func FlagDefault(v string) FlagOption {
	return func(f *invowkfile.Flag) {
		f.DefaultValue = v
	}
}

// FlagShorthand sets the flag's single-character shorthand.
func FlagShorthand(s string) FlagOption {
	return func(f *invowkfile.Flag) {
		f.Short = invowkfile.FlagShorthand(s)
	}
}

// FlagDescription sets the flag's description.
func FlagDescription(desc string) FlagOption {
	return func(f *invowkfile.Flag) {
		f.Description = desc
	}
}

// FlagType sets the flag's type.
func FlagType(t invowkfile.FlagType) FlagOption {
	return func(f *invowkfile.Flag) {
		f.Type = t
	}
}

// FlagValidation sets the flag's validation regex pattern.
func FlagValidation(pattern string) FlagOption {
	return func(f *invowkfile.Flag) {
		f.Validation = invowkfile.RegexPattern(pattern)
	}
}

// --- Arg Options ---

// WithArg adds a positional argument to the command.
func WithArg(name string, opts ...ArgOption) CommandOption {
	return func(c *invowkfile.Command) {
		a := invowkfile.Argument{Name: invowkfile.ArgumentName(name)}
		for _, opt := range opts {
			opt(&a)
		}
		c.Args = append(c.Args, a)
	}
}

// ArgRequired marks the argument as required.
func ArgRequired() ArgOption {
	return func(a *invowkfile.Argument) {
		a.Required = true
	}
}

// ArgDefault sets the argument's default value.
func ArgDefault(v string) ArgOption {
	return func(a *invowkfile.Argument) {
		a.DefaultValue = v
	}
}

// ArgVariadic marks the argument as variadic (accepts multiple values).
func ArgVariadic() ArgOption {
	return func(a *invowkfile.Argument) {
		a.Variadic = true
	}
}

// ArgDescription sets the argument's description.
func ArgDescription(desc string) ArgOption {
	return func(a *invowkfile.Argument) {
		a.Description = desc
	}
}

// ArgType sets the argument's type.
func ArgType(t invowkfile.ArgumentType) ArgOption {
	return func(a *invowkfile.Argument) {
		a.Type = t
	}
}

// ArgValidation sets the argument's validation regex pattern.
func ArgValidation(pattern string) ArgOption {
	return func(a *invowkfile.Argument) {
		a.Validation = invowkfile.RegexPattern(pattern)
	}
}
