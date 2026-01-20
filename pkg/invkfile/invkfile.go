// SPDX-License-Identifier: EPL-2.0

package invkfile

import (
	"path/filepath"
	goruntime "runtime"
)

const (
	// InvkfileName is the base name for invkfile configuration files
	InvkfileName = "invkfile"

	// InvkmodName is the base name for invkmod metadata files
	InvkmodName = "invkmod"
)

type (
	// Platform represents a target platform.
	// Alias for PlatformType for cleaner code.
	Platform = PlatformType

	// Invkfile represents command definitions from invkfile.cue.
	// Module metadata (module name, version, description, requires) is now in Invkmod.
	// This separation follows Go's pattern: invkmod.cue is like go.mod, invkfile.cue is like .go files.
	Invkfile struct {
		// DefaultShell overrides the default shell for native runtime
		DefaultShell string `json:"default_shell,omitempty"`
		// WorkDir specifies the default working directory for all commands
		// Can be absolute or relative to the invkfile location.
		// Forward slashes should be used for cross-platform compatibility.
		// Individual commands or implementations can override this with their own workdir.
		WorkDir string `json:"workdir,omitempty"`
		// Env contains global environment configuration for all commands (optional)
		// Root-level env is applied first (lowest priority from invkfile).
		// Command-level and implementation-level env override root-level env.
		Env *EnvConfig `json:"env,omitempty"`
		// DependsOn specifies global dependencies that apply to all commands (optional)
		// Root-level depends_on is combined with command-level and implementation-level depends_on.
		// Root-level dependencies are validated first (lowest priority in the merge order).
		// This is useful for defining shared prerequisites like required tools or capabilities
		// that apply to all commands in this invkfile.
		DependsOn *DependsOn `json:"depends_on,omitempty"`
		// Commands defines the available commands (invkfile field: 'cmds')
		Commands []Command `json:"cmds"`

		// FilePath stores the path where this invkfile was loaded from (not in CUE)
		FilePath string `json:"-"`
		// ModulePath stores the module directory path if this invkfile is from a module (not in CUE)
		// Empty string if not loaded from a module
		ModulePath string `json:"-"`
		// Metadata references the module metadata from invkmod.cue (not in CUE)
		// This is set when parsing a module via ParseModule
		Metadata *Invkmod `json:"-"`
	}
)

// GetCurrentHostOS returns the current operating system as Platform
func GetCurrentHostOS() Platform {
	switch goruntime.GOOS {
	case "linux":
		return PlatformLinux
	case "darwin":
		return PlatformMac // Returns "macos"
	case "windows":
		return PlatformWindows
	default:
		// Default to linux for unknown OS
		return PlatformLinux
	}
}

// GetCommand finds a command by its name (supports names with spaces like "test unit")
func (inv *Invkfile) GetCommand(name string) *Command {
	if name == "" {
		return nil
	}

	for i := range inv.Commands {
		if inv.Commands[i].Name == name {
			return &inv.Commands[i]
		}
	}

	return nil
}

// IsFromModule returns true if this invkfile was loaded from a module
func (inv *Invkfile) IsFromModule() bool {
	return inv.ModulePath != ""
}

// GetScriptBasePath returns the base path for resolving script file references.
// For module invkfiles, this is the module path.
// For regular invkfiles, this is the directory containing the invkfile.
func (inv *Invkfile) GetScriptBasePath() string {
	if inv.ModulePath != "" {
		return inv.ModulePath
	}
	return filepath.Dir(inv.FilePath)
}

// GetEffectiveWorkDir resolves the effective working directory for command execution.
// It follows the precedence hierarchy (highest to lowest):
//  1. CLI override (cliOverride parameter)
//  2. Implementation-level workdir (impl.WorkDir)
//  3. Command-level workdir (cmd.WorkDir)
//  4. Root-level workdir (inv.WorkDir)
//  5. Default: invkfile directory
//
// All workdir paths in CUE should use forward slashes for cross-platform compatibility.
// Relative paths are resolved against the invkfile location.
func (inv *Invkfile) GetEffectiveWorkDir(cmd *Command, impl *Implementation, cliOverride string) string {
	invkfileDir := inv.GetScriptBasePath()

	// resolve converts a workdir path from CUE format (forward slashes) to native format
	// and resolves relative paths against the invkfile directory.
	resolve := func(workdir string) string {
		if workdir == "" {
			return ""
		}
		// Convert forward slashes to native path separator
		nativePath := filepath.FromSlash(workdir)
		if filepath.IsAbs(nativePath) {
			return nativePath
		}
		return filepath.Join(invkfileDir, nativePath)
	}

	// Priority 1: CLI override
	if cliOverride != "" {
		return resolve(cliOverride)
	}

	// Priority 2: Implementation-level
	if impl != nil && impl.WorkDir != "" {
		return resolve(impl.WorkDir)
	}

	// Priority 3: Command-level
	if cmd != nil && cmd.WorkDir != "" {
		return resolve(cmd.WorkDir)
	}

	// Priority 4: Root-level
	if inv.WorkDir != "" {
		return resolve(inv.WorkDir)
	}

	// Priority 5: Default (invkfile directory)
	return invkfileDir
}

// GetFullCommandName returns the fully qualified command name with the module prefix.
// The format is "module cmdname" where cmdname may have spaces for subcommands.
// Returns empty string for the module prefix if no Metadata is set.
func (inv *Invkfile) GetFullCommandName(cmdName string) string {
	if inv.Metadata != nil {
		return inv.Metadata.Module + " " + cmdName
	}
	return cmdName
}

// GetModule returns the module identifier from Metadata, or empty string if not set.
func (inv *Invkfile) GetModule() string {
	if inv.Metadata != nil {
		return inv.Metadata.Module
	}
	return ""
}

// ListCommands returns all command names at the top level (with module prefix)
func (inv *Invkfile) ListCommands() []string {
	names := make([]string, len(inv.Commands))
	for i := range inv.Commands {
		names[i] = inv.GetFullCommandName(inv.Commands[i].Name)
	}
	return names
}

// FlattenCommands returns all commands keyed by their fully qualified names (with module prefix)
func (inv *Invkfile) FlattenCommands() map[string]*Command {
	result := make(map[string]*Command)
	for i := range inv.Commands {
		fullName := inv.GetFullCommandName(inv.Commands[i].Name)
		result[fullName] = &inv.Commands[i]
	}
	return result
}

// HasRootLevelDependencies returns true if the invkfile has root-level dependencies
func (inv *Invkfile) HasRootLevelDependencies() bool {
	if inv.DependsOn == nil {
		return false
	}
	return len(inv.DependsOn.Tools) > 0 || len(inv.DependsOn.Commands) > 0 || len(inv.DependsOn.Filepaths) > 0 || len(inv.DependsOn.Capabilities) > 0 || len(inv.DependsOn.CustomChecks) > 0 || len(inv.DependsOn.EnvVars) > 0
}
