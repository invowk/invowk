// SPDX-License-Identifier: EPL-2.0

package invkfile

import (
	"fmt"
	"strings"
)

// Command represents a single command that can be executed
type Command struct {
	// Name is the command identifier (can include spaces for subcommand-like behavior, e.g., "test unit")
	Name string `json:"name"`
	// Description provides help text for the command
	Description string `json:"description,omitempty"`
	// Implementations defines the executable implementations with platform/runtime constraints (required, at least one)
	Implementations []Implementation `json:"implementations"`
	// Env contains environment configuration for this command (optional)
	// Environment from files is loaded first, then vars override.
	// Command-level env is applied before implementation-level env.
	Env *EnvConfig `json:"env,omitempty"`
	// WorkDir specifies the working directory for command execution (optional)
	// Overrides root-level workdir but can be overridden by implementation-level workdir.
	// Can be absolute or relative to the invkfile location.
	// Forward slashes should be used for cross-platform compatibility.
	WorkDir string `json:"workdir,omitempty"`
	// DependsOn specifies dependencies that must be satisfied before running
	DependsOn *DependsOn `json:"depends_on,omitempty"`
	// Flags specifies command-line flags for this command
	// Note: 'env-file' (short 'e') and 'env-var' (short 'E') are reserved system flags and cannot be used.
	Flags []Flag `json:"flags,omitempty"`
	// Args specifies positional arguments for this command
	// Arguments are passed as environment variables: INVOWK_ARG_<NAME>
	// For variadic arguments: INVOWK_ARG_<NAME>_COUNT and INVOWK_ARG_<NAME>_1, _2, etc.
	Args []Argument `json:"args,omitempty"`
}

// GetImplForPlatformRuntime finds the script that matches the given platform and runtime
func (c *Command) GetImplForPlatformRuntime(platform Platform, runtime RuntimeMode) *Implementation {
	for i := range c.Implementations {
		s := &c.Implementations[i]
		if s.MatchesPlatform(platform) && s.HasRuntime(runtime) {
			return s
		}
	}
	return nil
}

// GetImplsForPlatform returns all scripts that can run on the given platform
func (c *Command) GetImplsForPlatform(platform Platform) []*Implementation {
	var result []*Implementation
	for i := range c.Implementations {
		if c.Implementations[i].MatchesPlatform(platform) {
			result = append(result, &c.Implementations[i])
		}
	}
	return result
}

// GetDefaultImplForPlatform returns the first script that matches the platform (default)
func (c *Command) GetDefaultImplForPlatform(platform Platform) *Implementation {
	scripts := c.GetImplsForPlatform(platform)
	if len(scripts) == 0 {
		return nil
	}
	return scripts[0]
}

// GetDefaultRuntimeForPlatform returns the default runtime for the given platform
// The default runtime is the first runtime of the first script that matches the platform
func (c *Command) GetDefaultRuntimeForPlatform(platform Platform) RuntimeMode {
	script := c.GetDefaultImplForPlatform(platform)
	if script == nil || len(script.Runtimes) == 0 {
		return RuntimeNative
	}
	return script.Runtimes[0].Name
}

// CanRunOnCurrentHost returns true if the command can run on the current host OS
func (c *Command) CanRunOnCurrentHost() bool {
	currentOS := GetCurrentHostOS()
	return len(c.GetImplsForPlatform(currentOS)) > 0
}

// GetSupportedPlatforms returns all platforms that this command supports
func (c *Command) GetSupportedPlatforms() []Platform {
	platformSet := make(map[Platform]bool)
	allPlatforms := []Platform{PlatformLinux, PlatformMac, PlatformWindows}

	for _, s := range c.Implementations {
		if len(s.Platforms) == 0 {
			// Implementation applies to all platforms
			for _, p := range allPlatforms {
				platformSet[p] = true
			}
		} else {
			for _, p := range s.Platforms {
				platformSet[p.Name] = true
			}
		}
	}

	var result []Platform
	for _, p := range allPlatforms {
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

	for _, s := range c.Implementations {
		if s.MatchesPlatform(platform) {
			for _, r := range s.Runtimes {
				if !runtimeSet[r.Name] {
					runtimeSet[r.Name] = true
					orderedRuntimes = append(orderedRuntimes, r.Name)
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
	for _, r := range c.GetAllowedRuntimesForPlatform(platform) {
		if r == runtime {
			return true
		}
	}
	return false
}

// ValidateScripts checks that there are no duplicate platform+runtime combinations
// Returns an error with a descriptive message if duplicates are found
func (c *Command) ValidateScripts() error {
	seen := make(map[PlatformRuntimeKey]int) // key -> script index (1-based for error messages)
	allPlatforms := []PlatformConfig{
		{Name: PlatformLinux},
		{Name: PlatformMac},
		{Name: PlatformWindows},
	}

	for i, s := range c.Implementations {
		platforms := s.Platforms
		if len(platforms) == 0 {
			platforms = allPlatforms // Applies to all platforms
		}

		for _, p := range platforms {
			for _, r := range s.Runtimes {
				key := PlatformRuntimeKey{Platform: p.Name, Runtime: r.Name}
				if existingIdx, exists := seen[key]; exists {
					return fmt.Errorf(
						"command '%s' has duplicate platform+runtime combination: platform=%s, runtime=%s (scripts #%d and #%d)",
						c.Name, p.Name, r.Name, existingIdx, i+1,
					)
				}
				seen[key] = i + 1
			}
		}
	}
	return nil
}

// HasDependencies returns true if the command has any dependencies (at command or script level)
func (c *Command) HasDependencies() bool {
	// Check command-level dependencies
	if c.DependsOn != nil {
		if len(c.DependsOn.Tools) > 0 || len(c.DependsOn.Commands) > 0 || len(c.DependsOn.Filepaths) > 0 || len(c.DependsOn.Capabilities) > 0 || len(c.DependsOn.CustomChecks) > 0 || len(c.DependsOn.EnvVars) > 0 {
			return true
		}
	}
	// Check implementation-level dependencies
	for _, s := range c.Implementations {
		if s.HasDependencies() {
			return true
		}
	}
	return false
}

// HasCommandLevelDependencies returns true if the command has command-level dependencies only
func (c *Command) HasCommandLevelDependencies() bool {
	if c.DependsOn == nil {
		return false
	}
	return len(c.DependsOn.Tools) > 0 || len(c.DependsOn.Commands) > 0 || len(c.DependsOn.Filepaths) > 0 || len(c.DependsOn.Capabilities) > 0 || len(c.DependsOn.CustomChecks) > 0 || len(c.DependsOn.EnvVars) > 0
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
