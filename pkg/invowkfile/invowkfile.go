// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"path/filepath"
	goruntime "runtime"

	"github.com/invowk/invowk/pkg/platform"
)

const (
	// InvowkfileName is the base name for invowkfile configuration files
	InvowkfileName = "invowkfile"

	// InvowkmodName is the base name for invowkmod metadata files
	InvowkmodName = "invowkmod"
)

type (
	// Platform represents a target platform.
	// Alias for PlatformType for cleaner code.
	Platform = PlatformType

	// Invowkfile represents command definitions from invowkfile.cue.
	// Module metadata (module name, version, description, requires) is now in Invowkmod.
	// This separation follows Go's pattern: invowkmod.cue is like go.mod, invowkfile.cue is like .go files.
	Invowkfile struct {
		// DefaultShell overrides the default shell for native runtime
		DefaultShell string `json:"default_shell,omitempty"`
		// WorkDir specifies the default working directory for all commands
		// Can be absolute or relative to the invowkfile location.
		// Forward slashes should be used for cross-platform compatibility.
		// Individual commands or implementations can override this with their own workdir.
		WorkDir string `json:"workdir,omitempty"`
		// Env contains global environment configuration for all commands (optional)
		// Root-level env is applied first (lowest priority from invowkfile).
		// Command-level and implementation-level env override root-level env.
		Env *EnvConfig `json:"env,omitempty"`
		// DependsOn specifies global dependencies that apply to all commands (optional)
		// Root-level depends_on is combined with command-level and implementation-level depends_on.
		// Root-level dependencies are validated first (lowest priority in the merge order).
		// This is useful for defining shared prerequisites like required tools or capabilities
		// that apply to all commands in this invowkfile.
		DependsOn *DependsOn `json:"depends_on,omitempty"`
		// Commands defines the available commands (invowkfile field: 'cmds')
		Commands []Command `json:"cmds"`

		// FilePath stores the path where this invowkfile was loaded from (not in CUE)
		FilePath string `json:"-"`
		// ModulePath stores the module directory path if this invowkfile is from a module (not in CUE)
		// Empty string if not loaded from a module
		ModulePath string `json:"-"`
		// Metadata references module identity/metadata attached during discovery.
		// It intentionally uses a local DTO to avoid coupling Invowkfile's core
		// command model to pkg/invowkmod internals.
		Metadata *ModuleMetadata `json:"-"`
	}
)

// CurrentPlatform returns the current operating system as Platform.
func CurrentPlatform() Platform {
	switch goruntime.GOOS {
	case "linux":
		return PlatformLinux
	case "darwin":
		return PlatformMac // Returns "macos"
	case platform.Windows:
		return PlatformWindows
	default:
		// Default to linux for unknown OS
		return PlatformLinux
	}
}

// GetCommand finds a command by its name (supports names with spaces like "test unit")
func (inv *Invowkfile) GetCommand(name string) *Command {
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

// IsFromModule returns true if this invowkfile was loaded from a module
func (inv *Invowkfile) IsFromModule() bool {
	return inv.ModulePath != ""
}

// GetScriptBasePath returns the base path for resolving script file references.
// For module invowkfiles, this is the module path.
// For regular invowkfiles, this is the directory containing the invowkfile.
func (inv *Invowkfile) GetScriptBasePath() string {
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
//  5. Default: invowkfile directory
//
// All workdir paths in CUE should use forward slashes for cross-platform compatibility.
// Relative paths are resolved against the invowkfile location.
func (inv *Invowkfile) GetEffectiveWorkDir(cmd *Command, impl *Implementation, cliOverride string) string {
	invowkfileDir := inv.GetScriptBasePath()

	// resolve converts a workdir path from CUE format (forward slashes) to native format
	// and resolves relative paths against the invowkfile directory.
	resolve := func(workdir string) string {
		if workdir == "" {
			return ""
		}
		// Convert forward slashes to native path separator
		nativePath := filepath.FromSlash(workdir)
		if filepath.IsAbs(nativePath) {
			return nativePath
		}
		return filepath.Join(invowkfileDir, nativePath)
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

	// Priority 5: Default (invowkfile directory)
	return invowkfileDir
}

// GetFullCommandName returns the fully qualified command name with the module prefix.
// The format is "module cmdname" where cmdname may have spaces for subcommands.
// Returns empty string for the module prefix if no Metadata is set.
func (inv *Invowkfile) GetFullCommandName(cmdName string) string {
	if inv.Metadata != nil {
		return string(inv.Metadata.Module) + " " + cmdName
	}
	return cmdName
}

// GetModule returns the module identifier from Metadata, or empty string if not set.
func (inv *Invowkfile) GetModule() string {
	if inv.Metadata != nil {
		return string(inv.Metadata.Module)
	}
	return ""
}

// ListCommands returns all command names at the top level (with module prefix)
func (inv *Invowkfile) ListCommands() []string {
	names := make([]string, len(inv.Commands))
	for i := range inv.Commands {
		names[i] = inv.GetFullCommandName(inv.Commands[i].Name)
	}
	return names
}

// FlattenCommands returns all commands keyed by their fully qualified names (with module prefix)
func (inv *Invowkfile) FlattenCommands() map[string]*Command {
	result := make(map[string]*Command)
	for i := range inv.Commands {
		fullName := inv.GetFullCommandName(inv.Commands[i].Name)
		result[fullName] = &inv.Commands[i]
	}
	return result
}

// HasRootLevelDependencies returns true if the invowkfile has root-level dependencies.
// Delegates to DependsOn.IsEmpty() to stay in sync if new dependency types are added.
func (inv *Invowkfile) HasRootLevelDependencies() bool {
	return inv.DependsOn != nil && !inv.DependsOn.IsEmpty()
}
