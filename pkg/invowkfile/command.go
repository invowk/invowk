// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// ErrInvalidCommandName is the sentinel error wrapped by InvalidCommandNameError.
var ErrInvalidCommandName = errors.New("invalid command name")

type (
	// CommandName represents a command identifier.
	// Command names can contain spaces for subcommand-like behavior (e.g., "test unit").
	// A valid command name is non-empty and not whitespace-only.
	CommandName string

	// InvalidCommandNameError is returned when a CommandName value is empty
	// or whitespace-only.
	InvalidCommandNameError struct {
		Value CommandName
	}

	// Command represents a single command that can be executed
	Command struct {
		// Name is the command identifier (can include spaces for subcommand-like behavior, e.g., "test unit")
		Name string `json:"name"`
		// Description provides help text for the command
		Description string `json:"description,omitempty"`
		// Category groups this command under a heading in 'invowk cmd' output (optional)
		Category string `json:"category,omitempty"`
		// Implementations defines the executable implementations with platform/runtime constraints (required, at least one)
		Implementations []Implementation `json:"implementations"`
		// Env contains environment configuration for this command (optional)
		// Environment from files is loaded first, then vars override.
		// Command-level env is applied before implementation-level env.
		Env *EnvConfig `json:"env,omitempty"`
		// WorkDir specifies the working directory for command execution (optional)
		// Overrides root-level workdir but can be overridden by implementation-level workdir.
		// Can be absolute or relative to the invowkfile location.
		// Forward slashes should be used for cross-platform compatibility.
		WorkDir string `json:"workdir,omitempty"`
		// DependsOn specifies dependencies that must be satisfied before running
		DependsOn *DependsOn `json:"depends_on,omitempty"`
		// Flags specifies command-line flags for this command.
		// Note: All flags starting with 'ivk-', 'invowk-', or 'i-' are reserved for system use.
		// Additionally, 'help' and 'version' are reserved built-in flags.
		Flags []Flag `json:"flags,omitempty"`
		// Args specifies positional arguments for this command
		// Arguments are passed as environment variables: INVOWK_ARG_<NAME>
		// For variadic arguments: INVOWK_ARG_<NAME>_COUNT and INVOWK_ARG_<NAME>_1, _2, etc.
		Args []Argument `json:"args,omitempty"`
		// Watch defines file-watching configuration for automatic re-execution (optional).
		// When defined, the command can be activated in watch mode via --ivk-watch.
		Watch *WatchConfig `json:"watch,omitempty"`
	}
)

// Error implements the error interface.
func (e *InvalidCommandNameError) Error() string {
	return fmt.Sprintf("invalid command name %q (must not be empty or whitespace-only)", e.Value)
}

// Unwrap returns ErrInvalidCommandName so callers can use errors.Is for programmatic detection.
func (e *InvalidCommandNameError) Unwrap() error { return ErrInvalidCommandName }

// IsValid returns whether the CommandName is a valid command identifier,
// and a list of validation errors if it is not.
func (n CommandName) IsValid() (bool, []error) {
	if strings.TrimSpace(string(n)) == "" {
		return false, []error{&InvalidCommandNameError{Value: n}}
	}
	return true, nil
}

// String returns the string representation of the CommandName.
func (n CommandName) String() string { return string(n) }

// GetImplForPlatformRuntime finds the implementation that matches the given platform and runtime.
func (c *Command) GetImplForPlatformRuntime(platform Platform, runtime RuntimeMode) *Implementation {
	for i := range c.Implementations {
		s := &c.Implementations[i]
		if s.MatchesPlatform(platform) && s.HasRuntime(runtime) {
			return s
		}
	}
	return nil
}

// GetImplsForPlatform returns all implementations that can run on the given platform.
func (c *Command) GetImplsForPlatform(platform Platform) []*Implementation {
	var result []*Implementation
	for i := range c.Implementations {
		if c.Implementations[i].MatchesPlatform(platform) {
			result = append(result, &c.Implementations[i])
		}
	}
	return result
}

// GetDefaultImplForPlatform returns the first implementation that matches the platform (default).
func (c *Command) GetDefaultImplForPlatform(platform Platform) *Implementation {
	impls := c.GetImplsForPlatform(platform)
	if len(impls) == 0 {
		return nil
	}
	return impls[0]
}

// GetDefaultRuntimeForPlatform returns the default runtime for the given platform.
// The default runtime is the first runtime of the first implementation that matches the platform.
func (c *Command) GetDefaultRuntimeForPlatform(platform Platform) RuntimeMode {
	impl := c.GetDefaultImplForPlatform(platform)
	if impl == nil || len(impl.Runtimes) == 0 {
		return RuntimeNative
	}
	return impl.Runtimes[0].Name
}

// CanRunOnCurrentHost returns true if the command can run on the current host OS
func (c *Command) CanRunOnCurrentHost() bool {
	currentOS := CurrentPlatform()
	return len(c.GetImplsForPlatform(currentOS)) > 0
}

// GetSupportedPlatforms returns all platforms that this command supports.
// Platforms are mandatory on each implementation, so this aggregates the explicitly declared platforms.
func (c *Command) GetSupportedPlatforms() []Platform {
	platformSet := make(map[Platform]bool)

	for i := range c.Implementations {
		for _, p := range c.Implementations[i].Platforms {
			platformSet[p.Name] = true
		}
	}

	var result []Platform
	for _, p := range AllPlatformNames() {
		if platformSet[p] {
			result = append(result, p)
		}
	}
	return result
}

// GetPlatformsString returns a comma-separated string of supported platforms
func (c *Command) GetPlatformsString() string {
	platforms := c.GetSupportedPlatforms()
	if len(platforms) == 0 {
		return ""
	}
	strs := make([]string, len(platforms))
	for i, p := range platforms {
		strs[i] = string(p)
	}
	return strings.Join(strs, ", ")
}

// GetAllowedRuntimesForPlatform returns all allowed runtimes for a given platform
func (c *Command) GetAllowedRuntimesForPlatform(platform Platform) []RuntimeMode {
	runtimeSet := make(map[RuntimeMode]bool)
	var orderedRuntimes []RuntimeMode

	for i := range c.Implementations {
		if c.Implementations[i].MatchesPlatform(platform) {
			for j := range c.Implementations[i].Runtimes {
				name := c.Implementations[i].Runtimes[j].Name
				if !runtimeSet[name] {
					runtimeSet[name] = true
					orderedRuntimes = append(orderedRuntimes, name)
				}
			}
		}
	}
	return orderedRuntimes
}

// GetRuntimesStringForPlatform returns a formatted string of runtimes for a platform with default highlighted
func (c *Command) GetRuntimesStringForPlatform(platform Platform) string {
	runtimes := c.GetAllowedRuntimesForPlatform(platform)
	if len(runtimes) == 0 {
		return ""
	}
	defaultRuntime := c.GetDefaultRuntimeForPlatform(platform)
	strs := make([]string, len(runtimes))
	for i, r := range runtimes {
		if r == defaultRuntime {
			strs[i] = string(r) + "*"
		} else {
			strs[i] = string(r)
		}
	}
	return strings.Join(strs, ", ")
}

// IsRuntimeAllowedForPlatform checks if the given runtime is allowed for the platform
func (c *Command) IsRuntimeAllowedForPlatform(platform Platform, runtime RuntimeMode) bool {
	return slices.Contains(c.GetAllowedRuntimesForPlatform(platform), runtime)
}

// ValidateImplementations checks that there are no duplicate platform+runtime combinations.
// Returns an error with a descriptive message if duplicates are found.
// Platforms are mandatory on each implementation, so this iterates the explicitly declared platforms.
func (c *Command) ValidateImplementations() error {
	seen := make(map[PlatformRuntimeKey]int) // key -> implementation index (1-based for error messages)

	for i := range c.Implementations {
		impl := &c.Implementations[i]

		for j := range impl.Platforms {
			for k := range impl.Runtimes {
				key := PlatformRuntimeKey{Platform: impl.Platforms[j].Name, Runtime: impl.Runtimes[k].Name}
				if existingIdx, exists := seen[key]; exists {
					return fmt.Errorf(
						"command '%s' has duplicate platform+runtime combination: platform=%s, runtime=%s (implementations #%d and #%d)",
						c.Name, impl.Platforms[j].Name, impl.Runtimes[k].Name, existingIdx, i+1,
					)
				}
				seen[key] = i + 1
			}
		}
	}
	return nil
}

// HasDependencies returns true if the command has any dependencies (at command or implementation level).
func (c *Command) HasDependencies() bool {
	if c.HasCommandLevelDependencies() {
		return true
	}
	for i := range c.Implementations {
		if c.Implementations[i].HasDependencies() {
			return true
		}
	}
	return false
}

// HasCommandLevelDependencies returns true if the command has command-level dependencies only.
// Delegates to DependsOn.IsEmpty() to stay in sync if new dependency types are added.
func (c *Command) HasCommandLevelDependencies() bool {
	return c.DependsOn != nil && !c.DependsOn.IsEmpty()
}

// GetCommandDependencies returns the list of command dependency names (from command level)
// For dependencies with alternatives, returns all alternatives flattened into a single list
func (c *Command) GetCommandDependencies() []string {
	if c.DependsOn == nil {
		return nil
	}
	var names []string
	for _, dep := range c.DependsOn.Commands {
		names = append(names, dep.Alternatives...)
	}
	return names
}
