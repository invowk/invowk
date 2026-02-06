// SPDX-License-Identifier: MPL-2.0

package invkmod

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"invowk-cli/internal/cueutil"
)

const (
	// MaxPathLength is the maximum allowed length for file paths.
	MaxPathLength = 4096

	// ModuleSuffix is the standard suffix for invowk module directories.
	ModuleSuffix = ".invkmod"

	// VendoredModulesDir is the directory name for vendored module dependencies.
	VendoredModulesDir = "invk_modules"
)

var (
	//go:embed invkmod_schema.cue
	invkmodSchema string

	// ErrInvkmodNotFound is returned when invkmod.cue is not found in a module directory.
	// Callers can check for this error using errors.Is(err, ErrInvkmodNotFound).
	ErrInvkmodNotFound = errors.New("invkmod.cue not found")
)

type (
	// ModuleCommands defines the typed command contract stored in Module.Commands.
	// This abstraction decouples module identity from invkfile command listing,
	// allowing Module to hold command access without depending on pkg/invkfile
	// parsing types. GetModule returns the module identifier from invkmod.cue.
	// ListCommands returns command names in no guaranteed order.
	ModuleCommands interface {
		GetModule() string
		ListCommands() []string
	}

	// ValidationIssue represents a single domain-level validation problem in a module.
	// Use ValidationIssue for problems that are collected and reported as a batch via
	// ValidationResult. Use error returns for I/O or infrastructure failures that
	// prevent validation from continuing.
	// Named "Issue" rather than "Error" because it semantically represents a validation
	// problem that may be collected, reported, and inspected - not just thrown.
	//
	//nolint:errname // Intentionally named Issue, not Error - semantic domain type
	ValidationIssue struct {
		// Type categorizes the issue (e.g., "structure", "naming", "invkfile")
		Type string `json:"-"`
		// Message describes the specific problem
		Message string `json:"-"`
		// Path is the relative path within the module where the issue was found (optional)
		Path string `json:"-"`
	}

	// ValidationResult contains the result of module validation.
	ValidationResult struct {
		// Valid is true if the module passed all validation checks
		Valid bool `json:"-"`
		// ModulePath is the absolute path to the validated module
		ModulePath string `json:"-"`
		// ModuleName is the extracted name from the folder (without .invkmod suffix)
		ModuleName string `json:"-"`
		// InvkmodPath is the path to the invkmod.cue within the module (required)
		InvkmodPath string `json:"-"`
		// InvkfilePath is the path to the invkfile.cue within the module (optional for library-only modules)
		InvkfilePath string `json:"-"`
		// IsLibraryOnly is true if the module has no invkfile.cue
		IsLibraryOnly bool `json:"-"`
		// Issues contains all validation problems found
		Issues []ValidationIssue `json:"-"`
	}

	// Module represents a loaded invowk module, ready for use.
	// This is the unified type combining filesystem structure with parsed content.
	Module struct {
		// Metadata is the parsed invkmod.cue content (always present after Load())
		Metadata *Invkmod `json:"-"`

		// Commands is the parsed invkfile.cue content (nil for library-only modules).
		Commands ModuleCommands `json:"-"`

		// Path is the absolute filesystem path to the module directory
		Path string `json:"-"`

		// IsLibraryOnly is true if the module has no invkfile.cue
		IsLibraryOnly bool `json:"-"`
	}

	// ModuleRequirement represents a dependency on another module from a Git repository.
	ModuleRequirement struct {
		// GitURL is the Git repository URL (HTTPS or SSH format).
		// Examples: "https://github.com/user/repo.git", "git@github.com:user/repo.git"
		GitURL string `json:"git_url"`
		// Version is the semver constraint for version selection.
		// Examples: "^1.2.0", "~1.2.0", ">=1.0.0 <2.0.0", "1.2.3"
		Version string `json:"version"`
		// Alias overrides the default namespace for imported commands (optional).
		// If not set, the namespace is: <module>@<resolved-version>
		Alias string `json:"alias,omitempty"`
		// Path specifies a subdirectory containing the module (optional).
		// Used for monorepos with multiple modules.
		Path string `json:"path,omitempty"`
	}

	// Invkmod represents module metadata from invkmod.cue.
	// This is analogous to Go's go.mod file - it contains module identity and dependencies.
	// Command definitions remain in invkfile.cue (separate file).
	Invkmod struct {
		// Module is a MANDATORY identifier for this module.
		// Acts as module identity and command namespace prefix.
		// Must start with a letter, contain only alphanumeric characters, with optional
		// dot-separated segments. RDNS format recommended (e.g., "io.invowk.sample", "com.example.mytools")
		// IMPORTANT: The module value MUST match the folder name prefix (before .invkmod)
		Module string `json:"module"`
		// Version specifies the module schema version (optional but recommended).
		// Current version: "1.0"
		Version string `json:"version,omitempty"`
		// Description provides a summary of this module's purpose (optional).
		Description string `json:"description,omitempty"`
		// Requires declares dependencies on other modules from Git repositories (optional).
		// Dependencies are resolved at module level.
		// All required modules are loaded and their commands made available.
		// IMPORTANT: Commands in this module can ONLY call:
		//   1. Commands from globally installed modules (~/.invowk/modules/)
		//   2. Commands from modules declared directly in THIS requires list
		// Commands CANNOT call transitive dependencies (dependencies of dependencies).
		Requires []ModuleRequirement `json:"requires,omitempty"`
		// FilePath stores the path where this invkmod.cue was loaded from (not in CUE)
		FilePath string `json:"-"`
	}

	// CommandScope defines what commands a module can access.
	// Commands in a module can ONLY call:
	//  1. Commands from the same module
	//  2. Commands from globally installed modules (~/.invowk/modules/)
	//  3. Commands from first-level requirements (direct dependencies in invkmod.cue:requires)
	//
	// Commands CANNOT call transitive dependencies (dependencies of dependencies).
	CommandScope struct {
		// ModuleID is the module identifier that owns this scope
		ModuleID string `json:"-"`
		// GlobalModules are commands from globally installed modules (always accessible)
		GlobalModules map[string]bool `json:"-"`
		// DirectDeps are module IDs from first-level requirements (from invkmod.cue:requires)
		DirectDeps map[string]bool `json:"-"`
	}
)

// Error implements the error interface for ValidationIssue.
func (v ValidationIssue) Error() string {
	if v.Path != "" {
		return fmt.Sprintf("[%s] %s: %s", v.Type, v.Path, v.Message)
	}
	return fmt.Sprintf("[%s] %s", v.Type, v.Message)
}

// AddIssue adds a validation issue to the result.
func (r *ValidationResult) AddIssue(issueType, message, path string) {
	r.Issues = append(r.Issues, ValidationIssue{
		Type:    issueType,
		Message: message,
		Path:    path,
	})
	r.Valid = false
}

// Name returns the module identifier from metadata.
// This is the value of the 'module' field in invkmod.cue.
func (m *Module) Name() string {
	if m.Metadata == nil {
		return ""
	}
	return m.Metadata.Module
}

// InvkmodPath returns the absolute path to invkmod.cue for this module.
func (m *Module) InvkmodPath() string {
	return filepath.Join(m.Path, "invkmod.cue")
}

// InvkfilePath returns the absolute path to invkfile.cue for this module.
// Returns empty string for library-only modules.
func (m *Module) InvkfilePath() string {
	if m.IsLibraryOnly {
		return ""
	}
	return filepath.Join(m.Path, "invkfile.cue")
}

// ResolveScriptPath resolves a script path relative to the module root.
// Script paths in modules should use forward slashes for cross-platform compatibility.
// This function converts the cross-platform path to the native format.
func (m *Module) ResolveScriptPath(scriptPath string) string {
	// Convert forward slashes to native path separator
	nativePath := filepath.FromSlash(scriptPath)

	// If already absolute, return as-is
	if filepath.IsAbs(nativePath) {
		return nativePath
	}

	// Resolve relative to module root
	return filepath.Join(m.Path, nativePath)
}

// ValidateScriptPath checks if a script path is valid for this module.
// Returns an error if the path is invalid (e.g., escapes module directory, is a symlink).
func (m *Module) ValidateScriptPath(scriptPath string) error {
	if scriptPath == "" {
		return fmt.Errorf("script path cannot be empty")
	}

	// Convert to native path
	nativePath := filepath.FromSlash(scriptPath)

	// Absolute paths are not allowed in modules
	if filepath.IsAbs(nativePath) {
		return fmt.Errorf("absolute paths are not allowed in modules; use paths relative to module root")
	}

	// Resolve the full path
	fullPath := filepath.Join(m.Path, nativePath)

	// Ensure the resolved path is within the module (prevent directory traversal)
	relPath, err := filepath.Rel(m.Path, fullPath)
	if err != nil {
		return fmt.Errorf("failed to resolve relative path: %w", err)
	}

	// Check for path escaping (e.g., "../something")
	if strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("script path '%s' escapes the module directory", scriptPath)
	}

	// Check if the path or any parent is a symlink
	if err := m.checkSymlinkSafety(fullPath); err != nil {
		return err
	}

	return nil
}

// ContainsPath checks if the given path is inside this module.
func (m *Module) ContainsPath(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	relPath, err := filepath.Rel(m.Path, absPath)
	if err != nil {
		return false
	}

	return !strings.HasPrefix(relPath, "..")
}

// GetInvkfileDir returns the directory containing the invkfile.
// For modules, this is always the module root.
func (m *Module) GetInvkfileDir() string {
	return m.Path
}

// checkSymlinkSafety verifies that a path doesn't contain symlinks that could escape the module.
func (m *Module) checkSymlinkSafety(path string) error {
	// Get the real path by resolving all symlinks
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		// If the file doesn't exist, that's fine - it'll be caught elsewhere
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot resolve symlinks in path: %w", err)
	}

	// Ensure the real path is still within the module
	moduleRealPath, err := filepath.EvalSymlinks(m.Path)
	if err != nil {
		return fmt.Errorf("cannot resolve module path: %w", err)
	}

	relPath, err := filepath.Rel(moduleRealPath, realPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("path resolves to location outside module directory (symlink escape)")
	}

	return nil
}

// NewCommandScope creates a CommandScope for a parsed module.
// globalModuleIDs should contain module IDs from ~/.invowk/modules/
// directRequirements should be the requires list from the module's invkmod.cue
func NewCommandScope(moduleID string, globalModuleIDs []string, directRequirements []ModuleRequirement) *CommandScope {
	scope := &CommandScope{
		ModuleID:      moduleID,
		GlobalModules: make(map[string]bool),
		DirectDeps:    make(map[string]bool),
	}

	for _, id := range globalModuleIDs {
		scope.GlobalModules[id] = true
	}

	for _, req := range directRequirements {
		// The direct dependency namespace uses either alias or the resolved module ID
		if req.Alias != "" {
			scope.DirectDeps[req.Alias] = true
		}
		// Note: The actual resolved module ID will be added during resolution
	}

	return scope
}

// CanCall checks if a command can call another command based on scope rules.
// Returns true if allowed, false with reason if not.
func (s *CommandScope) CanCall(targetCmd string) (allowed bool, reason string) {
	// Extract module prefix from command name (format: "module.name cmdname" or "module.name@version cmdname")
	targetModule := ExtractModuleFromCommand(targetCmd)

	// If no module prefix, it's a local command (always allowed)
	if targetModule == "" {
		return true, ""
	}

	// Check if target is from same module
	if targetModule == s.ModuleID {
		return true, ""
	}

	// Check if target is in global modules
	if s.GlobalModules[targetModule] {
		return true, ""
	}

	// Check if target is in direct dependencies
	if s.DirectDeps[targetModule] {
		return true, ""
	}

	return false, fmt.Sprintf(
		"command from module '%s' cannot call '%s': module '%s' is not accessible\n"+
			"  Commands can only call:\n"+
			"  - Commands from the same module (%s)\n"+
			"  - Commands from globally installed modules (~/.invowk/modules/)\n"+
			"  - Commands from direct dependencies declared in invkmod.cue:requires\n"+
			"  Add '%s' to your invkmod.cue requires list to use its commands",
		s.ModuleID, targetCmd, targetModule, s.ModuleID, targetModule)
}

// AddDirectDep adds a resolved direct dependency to the scope.
// This is called during resolution when we know the actual module ID.
func (s *CommandScope) AddDirectDep(moduleID string) {
	s.DirectDeps[moduleID] = true
}

// ExtractModuleFromCommand extracts the module prefix from a fully qualified command name.
// Returns empty string if no module prefix found.
// Examples:
//   - "io.invowk.sample hello" -> "io.invowk.sample"
//   - "utils@1.2.3 build" -> "utils@1.2.3"
//   - "build" -> ""
func ExtractModuleFromCommand(cmd string) string {
	// Command format: "module cmdname" where module may contain dots and @version
	parts := strings.SplitN(cmd, " ", 2)
	if len(parts) < 2 {
		// No space means it's either a local command or just a module with no command
		return ""
	}
	return parts[0]
}

// ParseInvkmod reads and parses module metadata from invkmod.cue at the given path.
func ParseInvkmod(path string) (*Invkmod, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read invkmod at %s: %w", path, err)
	}

	return ParseInvkmodBytes(data, path)
}

// ParseInvkmodBytes parses module metadata content from bytes.
// Uses cueutil.ParseAndDecodeString for the 3-step CUE parsing flow:
// compile schema → compile user data → validate and decode.
func ParseInvkmodBytes(data []byte, path string) (*Invkmod, error) {
	result, err := cueutil.ParseAndDecodeString[Invkmod](
		invkmodSchema,
		data,
		"#Invkmod",
		cueutil.WithFilename(path),
	)
	if err != nil {
		return nil, err
	}

	meta := result.Value
	meta.FilePath = path

	// Validate module requirement paths for security
	// [GO-ONLY] Path traversal prevention and cross-platform path handling require Go.
	// CUE cannot perform filesystem operations or cross-platform path normalization.
	for i, req := range meta.Requires {
		if req.Path != "" {
			if len(req.Path) > MaxPathLength {
				return nil, fmt.Errorf("requires[%d].path: too long (%d chars, max %d) in invkmod at %s", i, len(req.Path), MaxPathLength, path)
			}
			if strings.ContainsRune(req.Path, '\x00') {
				return nil, fmt.Errorf("requires[%d].path: contains null byte in invkmod at %s", i, path)
			}
			cleanPath := filepath.Clean(req.Path)
			if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
				return nil, fmt.Errorf("requires[%d].path: path traversal or absolute paths not allowed in invkmod at %s", i, path)
			}
		}
	}

	return meta, nil
}

// ParseModuleMetadataOnly reads and parses only the module metadata (invkmod.cue) from a module directory.
// This is useful when you only need module identity and dependencies, not commands.
// Returns ErrInvkmodNotFound if invkmod.cue doesn't exist.
func ParseModuleMetadataOnly(modulePath string) (*Invkmod, error) {
	invkmodPath := filepath.Join(modulePath, "invkmod.cue")
	if _, err := os.Stat(invkmodPath); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrInvkmodNotFound
		}
		return nil, fmt.Errorf("failed to check invkmod at %s: %w", invkmodPath, err)
	}
	return ParseInvkmod(invkmodPath)
}

// HasInvkfile checks if a module directory contains an invkfile.cue.
func HasInvkfile(modulePath string) bool {
	invkfilePath := filepath.Join(modulePath, "invkfile.cue")
	_, err := os.Stat(invkfilePath)
	return err == nil
}

// InvkfilePath returns the path to invkfile.cue in a module directory.
func InvkfilePath(modulePath string) string {
	return filepath.Join(modulePath, "invkfile.cue")
}

// InvkmodPath returns the path to invkmod.cue in a module directory.
//
//nolint:revive // Name is intentional for consistency with Module.InvkmodPath field/method
func InvkmodPath(modulePath string) string {
	return filepath.Join(modulePath, "invkmod.cue")
}
