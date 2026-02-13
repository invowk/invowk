// SPDX-License-Identifier: MPL-2.0

package contracts

// This file defines the contract for the consolidated test command builder.
// Replaces duplicated testCommand() / testCmd() functions.

// RuntimeType mirrors invowkfile.RuntimeType for this contract.
type RuntimeType string

const (
	RuntimeNative    RuntimeType = "native"
	RuntimeVirtual   RuntimeType = "virtual"
	RuntimeContainer RuntimeType = "container"
)

// CommandOption configures a test command.
// Apply options to customize beyond the minimal defaults.
type CommandOption func(*TestCommand)

// TestCommand is a simplified representation for contract documentation.
// Actual implementation uses *invowkfile.Command.
type TestCommand struct {
	Name            string
	Description     string
	Script          string
	Runtimes        []RuntimeType
	Platforms       []string
	Env             map[string]string
	Flags           []TestFlag
	Args            []TestArg
	WorkDir         string
	Dependencies    []any // Simplified for contract
	Implementations []TestImplementation
}

// TestFlag represents a command flag for testing.
type TestFlag struct {
	Name       string
	Shorthand  string
	Default    string
	EnvMapping string
	Required   bool
}

// TestArg represents a positional argument for testing.
type TestArg struct {
	Name     string
	Default  string
	Required bool
	Variadic bool
}

// TestImplementation represents a command implementation for testing.
type TestImplementation struct {
	Script    string
	Runtimes  []RuntimeType
	Platforms []string
}

// NewTestCommand creates a test command with the given name and options.
// By default, creates a command with:
//   - Single implementation with RuntimeNative
//   - Empty script (override with WithScript)
//   - No platform restrictions
//   - No flags or arguments
//
// Usage:
//
//	cmd := testutil.NewTestCommand("hello")
//	cmd := testutil.NewTestCommand("hello", testutil.WithScript("echo hello"))
//	cmd := testutil.NewTestCommand("hello",
//	    testutil.WithScript("echo $NAME"),
//	    testutil.WithRuntime(testutil.RuntimeVirtual),
//	    testutil.WithEnv("NAME", "world"),
//	)
func NewTestCommand(name string, opts ...CommandOption) *TestCommand {
	cmd := &TestCommand{
		Name:     name,
		Runtimes: []RuntimeType{RuntimeNative},
		Env:      make(map[string]string),
	}
	for _, opt := range opts {
		opt(cmd)
	}
	return cmd
}

// --- Command Options ---

// WithScript sets the implementation script.
func WithScript(script string) CommandOption {
	return func(c *TestCommand) {
		c.Script = script
	}
}

// WithDescription sets the command description.
func WithDescription(desc string) CommandOption {
	return func(c *TestCommand) {
		c.Description = desc
	}
}

// WithRuntime sets a single runtime.
func WithRuntime(r RuntimeType) CommandOption {
	return func(c *TestCommand) {
		c.Runtimes = []RuntimeType{r}
	}
}

// WithRuntimes sets multiple runtimes.
func WithRuntimes(rs ...RuntimeType) CommandOption {
	return func(c *TestCommand) {
		c.Runtimes = rs
	}
}

// WithPlatform adds a platform constraint.
func WithPlatform(p string) CommandOption {
	return func(c *TestCommand) {
		c.Platforms = append(c.Platforms, p)
	}
}

// WithPlatforms sets multiple platform constraints.
func WithPlatforms(ps ...string) CommandOption {
	return func(c *TestCommand) {
		c.Platforms = ps
	}
}

// WithEnv adds an environment variable to the command.
func WithEnv(key, value string) CommandOption {
	return func(c *TestCommand) {
		c.Env[key] = value
	}
}

// WithWorkDir sets the working directory.
func WithWorkDir(dir string) CommandOption {
	return func(c *TestCommand) {
		c.WorkDir = dir
	}
}

// --- Flag Options ---

// FlagOption configures a test command flag.
type FlagOption func(*TestFlag)

// WithFlag adds a flag to the command.
func WithFlag(name string, opts ...FlagOption) CommandOption {
	return func(c *TestCommand) {
		f := TestFlag{Name: name}
		for _, opt := range opts {
			opt(&f)
		}
		c.Flags = append(c.Flags, f)
	}
}

// FlagRequired marks the flag as required.
func FlagRequired() FlagOption {
	return func(f *TestFlag) {
		f.Required = true
	}
}

// FlagDefault sets the flag's default value.
func FlagDefault(v string) FlagOption {
	return func(f *TestFlag) {
		f.Default = v
	}
}

// FlagShorthand sets the flag's single-character shorthand.
func FlagShorthand(s string) FlagOption {
	return func(f *TestFlag) {
		f.Shorthand = s
	}
}

// FlagEnvMapping sets the environment variable that can provide the flag value.
func FlagEnvMapping(env string) FlagOption {
	return func(f *TestFlag) {
		f.EnvMapping = env
	}
}

// --- Arg Options ---

// ArgOption configures a test command argument.
type ArgOption func(*TestArg)

// WithArg adds a positional argument to the command.
func WithArg(name string, opts ...ArgOption) CommandOption {
	return func(c *TestCommand) {
		a := TestArg{Name: name}
		for _, opt := range opts {
			opt(&a)
		}
		c.Args = append(c.Args, a)
	}
}

// ArgRequired marks the argument as required.
func ArgRequired() ArgOption {
	return func(a *TestArg) {
		a.Required = true
	}
}

// ArgDefault sets the argument's default value.
func ArgDefault(v string) ArgOption {
	return func(a *TestArg) {
		a.Default = v
	}
}

// ArgVariadic marks the argument as variadic (accepts multiple values).
func ArgVariadic() ArgOption {
	return func(a *TestArg) {
		a.Variadic = true
	}
}

// --- Implementation Options ---

// WithImplementation adds a custom implementation.
// Use when you need multiple implementations or non-default configuration.
func WithImplementation(script string, runtimes []RuntimeType, platforms []string) CommandOption {
	return func(c *TestCommand) {
		c.Implementations = append(c.Implementations, TestImplementation{
			Script:    script,
			Runtimes:  runtimes,
			Platforms: platforms,
		})
	}
}
