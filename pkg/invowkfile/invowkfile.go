// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"github.com/invowk/invowk/pkg/fspath"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/platform"
)

const (
	// InvowkfileName is the base name for invowkfile configuration files
	InvowkfileName = "invowkfile"

	// InvowkmodName is the base name for invowkmod metadata files
	InvowkmodName = "invowkmod"
)

var (
	// ErrInvalidShellPath is the sentinel error wrapped by InvalidShellPathError.
	ErrInvalidShellPath = errors.New("invalid shell path")

	// ErrInvalidInvowkfile is the sentinel error wrapped by InvalidInvowkfileError.
	ErrInvalidInvowkfile = errors.New("invalid invowkfile")
)

type (
	// ShellPath represents a filesystem path to a shell executable.
	// The zero value ("") is valid and means "use system default shell".
	// Non-zero values must not be whitespace-only.
	ShellPath string

	// InvalidShellPathError is returned when a ShellPath value is whitespace-only.
	// It wraps ErrInvalidShellPath for errors.Is() compatibility.
	InvalidShellPathError struct {
		Value ShellPath
	}

	// Platform represents a target platform.
	// Alias for PlatformType for cleaner code.
	Platform = PlatformType

	// InvalidInvowkfileError is returned when an Invowkfile has invalid fields.
	// It wraps ErrInvalidInvowkfile for errors.Is() compatibility and collects
	// field-level validation errors.
	InvalidInvowkfileError struct {
		FieldErrors []error
	}

	//goplint:validate-all
	//
	// Invowkfile represents command definitions from invowkfile.cue.
	// Module metadata (module name, version, description, requires) is now in Invowkmod.
	// This separation follows Go's pattern: invowkmod.cue is like go.mod, invowkfile.cue is like .go files.
	//nolint:recvcheck // DDD Validate() (value) + existing methods (pointer)
	Invowkfile struct {
		// DefaultShell overrides the default shell for native runtime
		DefaultShell ShellPath `json:"default_shell,omitempty"`
		// WorkDir specifies the default working directory for all commands
		// Can be absolute or relative to the invowkfile location.
		// Forward slashes should be used for cross-platform compatibility.
		// Individual commands or implementations can override this with their own workdir.
		WorkDir WorkDir `json:"workdir,omitempty"`
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
		FilePath FilesystemPath `json:"-"`
		// ModulePath stores the module directory path if this invowkfile is from a module (not in CUE)
		// Empty value if not loaded from a module
		ModulePath FilesystemPath `json:"-"`
		// Metadata references module identity/metadata attached during discovery.
		// It intentionally uses a local DTO to avoid coupling Invowkfile's core
		// command model to pkg/invowkmod internals.
		Metadata *ModuleMetadata `json:"-"`
	}
)

// Error implements the error interface for InvalidShellPathError.
func (e *InvalidShellPathError) Error() string {
	return fmt.Sprintf("invalid shell path %q (must not be whitespace-only)", e.Value)
}

// Unwrap returns ErrInvalidShellPath for errors.Is() compatibility.
func (e *InvalidShellPathError) Unwrap() error { return ErrInvalidShellPath }

// Validate returns nil if the ShellPath is valid, or a validation error if not.
// The zero value ("") is valid — it means "use system default shell".
// Non-zero values must not be whitespace-only.
func (s ShellPath) Validate() error {
	if s == "" {
		return nil
	}
	if strings.TrimSpace(string(s)) == "" {
		return &InvalidShellPathError{Value: s}
	}
	return nil
}

// String returns the string representation of the ShellPath.
func (s ShellPath) String() string { return string(s) }

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

// ValidateFields returns nil if all typed fields in the Invowkfile are structurally valid,
// or an error collecting all field-level validation failures.
// This is the DDD Validate() equivalent; it cannot be named Validate() because
// *Invowkfile already has a Validate(opts ...ValidateOption) ValidationErrors method
// in validation.go that runs the full composite validation pipeline.
// Delegates to DefaultShell (zero-valid), WorkDir (zero-valid), Env (non-nil),
// DependsOn (non-nil), each Command, FilePath (non-empty), and ModulePath (non-empty).
func (inv Invowkfile) ValidateFields() error {
	var errs []error
	inv.appendBaseValidationErrors(&errs)
	inv.appendCommandValidationErrors(&errs)
	inv.appendPathValidationErrors(&errs)
	if len(errs) > 0 {
		return &InvalidInvowkfileError{FieldErrors: errs}
	}
	return nil
}

// Error implements the error interface for InvalidInvowkfileError.
func (e *InvalidInvowkfileError) Error() string {
	return fmt.Sprintf("invalid invowkfile: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidInvowkfile for errors.Is() compatibility.
func (e *InvalidInvowkfileError) Unwrap() error { return ErrInvalidInvowkfile }

// GetCommand finds a command by its name (supports names with spaces like "test unit")
func (inv *Invowkfile) GetCommand(name CommandName) *Command {
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
func (inv *Invowkfile) GetScriptBasePath() FilesystemPath {
	if inv.ModulePath != "" {
		return inv.ModulePath
	}
	return fspath.Dir(inv.FilePath)
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
func (inv *Invowkfile) GetEffectiveWorkDir(cmd *Command, impl *Implementation, cliOverride WorkDir) FilesystemPath {
	invowkfileDir := inv.GetScriptBasePath()

	// resolve converts a workdir path from CUE format (forward slashes) to native format
	// and resolves relative paths against the invowkfile directory.
	resolve := func(workdir string) FilesystemPath {
		if workdir == "" {
			return ""
		}
		// Convert forward slashes to native path separator
		nativePath := filepath.FromSlash(workdir)
		if filepath.IsAbs(nativePath) {
			return FilesystemPath(nativePath) //goplint:ignore -- OS path from filepath.IsAbs guard
		}
		return fspath.JoinStr(invowkfileDir, nativePath)
	}

	// Priority 1: CLI override
	if cliOverride != "" {
		return resolve(string(cliOverride))
	}

	// Priority 2: Implementation-level
	if impl != nil && impl.WorkDir != "" {
		return resolve(string(impl.WorkDir))
	}

	// Priority 3: Command-level
	if cmd != nil && cmd.WorkDir != "" {
		return resolve(string(cmd.WorkDir))
	}

	// Priority 4: Root-level
	if inv.WorkDir != "" {
		return resolve(string(inv.WorkDir))
	}

	// Priority 5: Default (invowkfile directory)
	return invowkfileDir
}

// GetFullCommandName returns the fully qualified command name with the module prefix.
// The format is "module cmdname" where cmdname may have spaces for subcommands.
// Returns the bare command name if no Metadata is set.
func (inv *Invowkfile) GetFullCommandName(cmdName CommandName) CommandName {
	if inv.Metadata != nil {
		return CommandName(string(inv.Metadata.Module()) + " " + string(cmdName)) //goplint:ignore -- composed from validated module + command name
	}
	return cmdName
}

// GetModule returns the module identifier from Metadata, or empty ModuleID if not set.
func (inv *Invowkfile) GetModule() invowkmod.ModuleID {
	if inv.Metadata != nil {
		return inv.Metadata.Module()
	}
	return ""
}

// ListCommands returns all command names at the top level (with module prefix).
// Returns []string to satisfy the invowkmod.ModuleCommands interface contract,
// which lives in a separate package that cannot depend on invowkfile types.
func (inv *Invowkfile) ListCommands() []string {
	names := make([]string, len(inv.Commands))
	for i := range inv.Commands {
		names[i] = string(inv.GetFullCommandName(inv.Commands[i].Name))
	}
	return names
}

// FlattenCommands returns all commands keyed by their fully qualified names (with module prefix)
func (inv *Invowkfile) FlattenCommands() map[CommandName]*Command {
	result := make(map[CommandName]*Command)
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

func (inv Invowkfile) appendBaseValidationErrors(errs *[]error) {
	if err := inv.DefaultShell.Validate(); err != nil {
		*errs = append(*errs, err)
	}
	if inv.WorkDir != "" {
		if err := inv.WorkDir.Validate(); err != nil {
			*errs = append(*errs, err)
		}
	}
	if inv.Env != nil {
		if err := inv.Env.Validate(); err != nil {
			*errs = append(*errs, err)
		}
	}
	if inv.DependsOn != nil {
		if err := inv.DependsOn.Validate(); err != nil {
			*errs = append(*errs, err)
		}
	}
}

func (inv Invowkfile) appendCommandValidationErrors(errs *[]error) {
	for i := range inv.Commands {
		if err := inv.Commands[i].Validate(); err != nil {
			*errs = append(*errs, err)
		}
	}
}

func (inv Invowkfile) appendPathValidationErrors(errs *[]error) {
	if inv.FilePath != "" {
		if err := inv.FilePath.Validate(); err != nil {
			*errs = append(*errs, err)
		}
	}
	if inv.ModulePath != "" {
		if err := inv.ModulePath.Validate(); err != nil {
			*errs = append(*errs, err)
		}
	}
}
