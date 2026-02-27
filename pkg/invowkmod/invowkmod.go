// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	slashpath "path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/invowk/invowk/pkg/cueutil"
	"github.com/invowk/invowk/pkg/types"
)

const (
	// MaxPathLength is the maximum allowed length for file paths.
	MaxPathLength = 4096

	// ModuleSuffix is the standard suffix for invowk module directories.
	ModuleSuffix = ".invowkmod"

	// VendoredModulesDir is the directory name for vendored module dependencies.
	VendoredModulesDir = "invowk_modules"

	// MaxModuleIDLength is the maximum allowed length for a module identifier.
	// This mirrors the CUE schema constraint: strings.MaxRunes(256).
	MaxModuleIDLength = 256

	// IssueTypeStructure categorizes structural validation issues (missing files, wrong layout).
	IssueTypeStructure ValidationIssueType = "structure"
	// IssueTypeNaming categorizes naming convention violations.
	IssueTypeNaming ValidationIssueType = "naming"
	// IssueTypeInvowkmod categorizes invowkmod.cue parsing or content issues.
	IssueTypeInvowkmod ValidationIssueType = "invowkmod"
	// IssueTypeSecurity categorizes security concerns (symlinks, path escapes).
	IssueTypeSecurity ValidationIssueType = "security"
	// IssueTypeCompatibility categorizes cross-platform compatibility issues.
	IssueTypeCompatibility ValidationIssueType = "compatibility"
	// IssueTypeInvowkfile categorizes invowkfile.cue parsing or content issues.
	IssueTypeInvowkfile ValidationIssueType = "invowkfile"
	// IssueTypeCommandTree categorizes command tree validation issues.
	IssueTypeCommandTree ValidationIssueType = "command_tree"
)

var (
	//go:embed invowkmod_schema.cue
	invowkmodSchema string

	// ErrInvowkmodNotFound is returned when invowkmod.cue is not found in a module directory.
	// Callers can check for this error using errors.Is(err, ErrInvowkmodNotFound).
	ErrInvowkmodNotFound = errors.New("invowkmod.cue not found")

	// ErrInvalidValidationIssueType is returned when a ValidationIssueType value is not one of the defined issue types.
	ErrInvalidValidationIssueType = errors.New("invalid validation issue type")

	// ErrInvalidModuleID is returned when a ModuleID value does not match the required format.
	ErrInvalidModuleID = errors.New("invalid module ID")

	// ErrInvalidModuleAlias is returned when a ModuleAlias value is whitespace-only.
	ErrInvalidModuleAlias = errors.New("invalid module alias")

	// ErrInvalidSubdirectoryPath is returned when a SubdirectoryPath value contains
	// path traversal or absolute paths.
	ErrInvalidSubdirectoryPath = errors.New("invalid subdirectory path")

	// ErrInvalidInvowkmod is the sentinel error wrapped by InvalidInvowkmodError.
	ErrInvalidInvowkmod = errors.New("invalid invowkmod")

	// moduleIDPattern validates the ModuleID format: starts with a letter, alphanumeric segments
	// separated by dots. This mirrors the CUE schema constraint in invowkmod_schema.cue.
	moduleIDPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*(\.[a-zA-Z][a-zA-Z0-9]*)*$`)
)

type (
	// ModuleID is a typed identifier for invowk modules (e.g., "io.invowk.sample").
	// Using a named type prevents accidental confusion with other string parameters
	// like command names, source IDs, or file paths.
	ModuleID string

	// InvalidModuleIDError is returned when a ModuleID value does not match the required format.
	// It wraps ErrInvalidModuleID for errors.Is() compatibility.
	InvalidModuleIDError struct {
		Value ModuleID
	}

	// ModuleAlias represents an optional namespace alias for imported module commands.
	// The zero value ("") is valid and means "no alias" (default namespace is used).
	// Non-zero values must not be whitespace-only.
	ModuleAlias string

	// InvalidModuleAliasError is returned when a ModuleAlias value is whitespace-only.
	// It wraps ErrInvalidModuleAlias for errors.Is() compatibility.
	InvalidModuleAliasError struct {
		Value ModuleAlias
	}

	// SubdirectoryPath represents a relative path to a subdirectory within a repository.
	// The zero value ("") is valid and means "repository root".
	// Non-zero values must not contain path traversal (..) or absolute paths.
	SubdirectoryPath string

	// InvalidSubdirectoryPathError is returned when a SubdirectoryPath value contains
	// path traversal or absolute paths.
	// It wraps ErrInvalidSubdirectoryPath for errors.Is() compatibility.
	InvalidSubdirectoryPathError struct {
		Value  SubdirectoryPath
		Reason string
	}

	// InvalidInvowkmodError is returned when an Invowkmod has invalid fields.
	// It wraps ErrInvalidInvowkmod for errors.Is() compatibility and collects
	// field-level validation errors from Module, Version, Description, and Requires.
	InvalidInvowkmodError struct {
		FieldErrors []error
	}

	// ModuleCommands defines the typed command contract stored in Module.Commands.
	// This abstraction decouples module identity from invowkfile command listing,
	// allowing Module to hold command access without depending on pkg/invowkfile
	// parsing types. GetModule returns the module identifier from invowkmod.cue.
	// ListCommands returns command names in no guaranteed order.
	ModuleCommands interface {
		GetModule() string
		ListCommands() []string
	}

	// ValidationIssueType categorizes module validation issues.
	ValidationIssueType string

	// InvalidValidationIssueTypeError is returned when a ValidationIssueType value is not recognized.
	// It wraps ErrInvalidValidationIssueType for errors.Is() compatibility.
	InvalidValidationIssueTypeError struct {
		Value ValidationIssueType
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
		// Type categorizes the issue (structure, naming, invowkmod, security, compatibility).
		Type ValidationIssueType `json:"-"`
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
		ModulePath types.FilesystemPath `json:"-"`
		// ModuleName is the extracted name from the folder (without .invowkmod suffix)
		ModuleName ModuleShortName `json:"-"`
		// InvowkmodPath is the path to the invowkmod.cue within the module (required)
		InvowkmodPath types.FilesystemPath `json:"-"`
		// InvowkfilePath is the path to the invowkfile.cue within the module (optional for library-only modules)
		InvowkfilePath types.FilesystemPath `json:"-"`
		// IsLibraryOnly is true if the module has no invowkfile.cue
		IsLibraryOnly bool `json:"-"`
		// Issues contains all validation problems found
		Issues []ValidationIssue `json:"-"`
	}

	// Module represents a loaded invowk module, ready for use.
	// This is the unified type combining filesystem structure with parsed content.
	Module struct {
		// Metadata is the parsed invowkmod.cue content (always present after Load())
		Metadata *Invowkmod `json:"-"`

		// Commands is the parsed invowkfile.cue content (nil for library-only modules).
		Commands ModuleCommands `json:"-"`

		// Path is the absolute filesystem path to the module directory
		Path types.FilesystemPath `json:"-"`

		// IsLibraryOnly is true if the module has no invowkfile.cue
		IsLibraryOnly bool `json:"-"`
	}

	//goplint:validate-all
	//
	// ModuleRequirement represents a dependency on another module from a Git repository.
	ModuleRequirement struct {
		// GitURL is the Git repository URL (HTTPS or SSH format).
		// Examples: "https://github.com/user/repo.git", "git@github.com:user/repo.git"
		GitURL GitURL `json:"git_url"`
		// Version is the semver constraint for version selection.
		// Examples: "^1.2.0", "~1.2.0", ">=1.0.0 <2.0.0", "1.2.3"
		Version SemVerConstraint `json:"version"`
		// Alias overrides the default namespace for imported commands (optional).
		// If not set, the namespace is: <module>@<resolved-version>
		Alias ModuleAlias `json:"alias,omitempty"`
		// Path specifies a subdirectory containing the module (optional).
		// Used for monorepos with multiple modules.
		Path SubdirectoryPath `json:"path,omitempty"`
	}

	//goplint:validate-all
	//
	// Invowkmod represents module metadata from invowkmod.cue.
	// This is analogous to Go's go.mod file - it contains module identity and dependencies.
	// Command definitions remain in invowkfile.cue (separate file).
	Invowkmod struct {
		// Module is a MANDATORY identifier for this module.
		// Acts as module identity and command namespace prefix.
		// Must start with a letter, contain only alphanumeric characters, with optional
		// dot-separated segments. RDNS format recommended (e.g., "io.invowk.sample", "com.example.mytools")
		// IMPORTANT: The module value MUST match the folder name prefix (before .invowkmod)
		Module ModuleID `json:"module"`
		// Version specifies the module version using semantic versioning (mandatory).
		// Format: MAJOR.MINOR.PATCH with optional pre-release label (e.g., "1.0.0", "2.1.0-alpha.1").
		// No "v" prefix, no build metadata, no leading zeros on numeric segments.
		Version SemVer `json:"version"`
		// Description provides a summary of this module's purpose (optional).
		Description types.DescriptionText `json:"description,omitempty"`
		// Requires declares dependencies on other modules from Git repositories (optional).
		// Dependencies are resolved at module level.
		// All required modules are loaded and their commands made available.
		// IMPORTANT: Commands in this module can ONLY call:
		//   1. Commands from globally installed modules (~/.invowk/modules/)
		//   2. Commands from modules declared directly in THIS requires list
		// Commands CANNOT call transitive dependencies (dependencies of dependencies).
		Requires []ModuleRequirement `json:"requires,omitempty"`
		// FilePath stores the path where this invowkmod.cue was loaded from (not in CUE)
		FilePath types.FilesystemPath `json:"-"`
	}

	// CommandScope defines what commands a module can access.
	// Commands in a module can ONLY call:
	//  1. Commands from the same module
	//  2. Commands from globally installed modules (~/.invowk/modules/)
	//  3. Commands from first-level requirements (direct dependencies in invowkmod.cue:requires)
	//
	// Commands CANNOT call transitive dependencies (dependencies of dependencies).
	CommandScope struct {
		// ModuleID is the module identifier that owns this scope
		ModuleID ModuleID `json:"-"`
		// GlobalModules are commands from globally installed modules (always accessible)
		GlobalModules map[ModuleID]bool `json:"-"`
		// DirectDeps are module IDs from first-level requirements (from invowkmod.cue:requires)
		DirectDeps map[ModuleID]bool `json:"-"`
	}
)

// Error implements the error interface for InvalidModuleIDError.
func (e *InvalidModuleIDError) Error() string {
	return fmt.Sprintf(
		"invalid module ID %q: must match format 'segment.segment...' "+
			"where each segment starts with a letter followed by alphanumeric characters, max %d characters",
		string(e.Value), MaxModuleIDLength,
	)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidModuleIDError) Unwrap() error {
	return ErrInvalidModuleID
}

//goplint:nonzero

// Validate returns nil if the ModuleID matches the required RDNS format,
// or an error describing the validation failure. The format requires:
// starts with a letter, alphanumeric segments separated by dots, max 256 runes.
// This mirrors the CUE schema constraint in invowkmod_schema.cue.
func (m ModuleID) Validate() error {
	s := string(m)
	if s == "" || len([]rune(s)) > MaxModuleIDLength || !moduleIDPattern.MatchString(s) {
		return &InvalidModuleIDError{Value: m}
	}

	return nil
}

// String returns the string representation of the ModuleID.
func (m ModuleID) String() string { return string(m) }

// Validate returns nil if the Invowkmod has valid fields, or an error
// collecting all field-level validation failures.
// It delegates to Module.Validate(), Version.Validate(), and each
// Requires entry's Validate(). Description and FilePath are validated
// only when non-empty (their zero values are valid).
func (m Invowkmod) Validate() error {
	var errs []error
	if err := m.Module.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := m.Version.Validate(); err != nil {
		errs = append(errs, err)
	}
	if m.Description != "" {
		if err := m.Description.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	for _, req := range m.Requires {
		if err := req.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if m.FilePath != "" {
		if err := m.FilePath.Validate(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return &InvalidInvowkmodError{FieldErrors: errs}
	}
	return nil
}

// Error implements the error interface for InvalidInvowkmodError.
func (e *InvalidInvowkmodError) Error() string {
	return fmt.Sprintf("invalid invowkmod: %d field error(s)", len(e.FieldErrors))
}

// Unwrap returns ErrInvalidInvowkmod for errors.Is() compatibility.
func (e *InvalidInvowkmodError) Unwrap() error { return ErrInvalidInvowkmod }

// Error implements the error interface for InvalidModuleAliasError.
func (e *InvalidModuleAliasError) Error() string {
	return fmt.Sprintf("invalid module alias %q (must not be whitespace-only)", e.Value)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidModuleAliasError) Unwrap() error {
	return ErrInvalidModuleAlias
}

// Validate returns nil if the ModuleAlias is valid, or an error
// describing the validation failure.
// The zero value ("") is valid — it means "no alias".
// Non-zero values must not be whitespace-only.
func (a ModuleAlias) Validate() error {
	if a == "" {
		return nil
	}
	if strings.TrimSpace(string(a)) == "" {
		return &InvalidModuleAliasError{Value: a}
	}
	return nil
}

// String returns the string representation of the ModuleAlias.
func (a ModuleAlias) String() string { return string(a) }

// Error implements the error interface for InvalidSubdirectoryPathError.
func (e *InvalidSubdirectoryPathError) Error() string {
	return fmt.Sprintf("invalid subdirectory path %q: %s", e.Value, e.Reason)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidSubdirectoryPathError) Unwrap() error {
	return ErrInvalidSubdirectoryPath
}

// Validate returns nil if the SubdirectoryPath is valid, or an error
// describing the validation failure.
// The zero value ("") is valid — it means "repository root".
// Non-zero values must not contain path traversal (..) or absolute paths.
func (p SubdirectoryPath) Validate() error {
	if p == "" {
		return nil
	}
	s := string(p)
	if len(s) > MaxPathLength {
		return &InvalidSubdirectoryPathError{
			Value:  p,
			Reason: fmt.Sprintf("too long (%d chars, max %d)", len(s), MaxPathLength),
		}
	}
	if strings.ContainsRune(s, '\x00') {
		return &InvalidSubdirectoryPathError{
			Value:  p,
			Reason: "contains null byte",
		}
	}
	// SubdirectoryPath semantics are cross-platform and repository-relative.
	// Normalize separators first so Windows-style inputs are validated consistently
	// on all hosts (Linux/macOS/Windows).
	cleanPath := slashpath.Clean(strings.ReplaceAll(s, "\\", "/"))
	if strings.HasPrefix(cleanPath, "/") {
		return &InvalidSubdirectoryPathError{
			Value:  p,
			Reason: "absolute paths not allowed",
		}
	}
	if len(cleanPath) >= 2 && cleanPath[1] == ':' {
		first := cleanPath[0]
		if (first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z') {
			return &InvalidSubdirectoryPathError{
				Value:  p,
				Reason: "absolute paths not allowed",
			}
		}
	}
	if cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		return &InvalidSubdirectoryPathError{
			Value:  p,
			Reason: "path traversal not allowed",
		}
	}
	return nil
}

// String returns the string representation of the SubdirectoryPath.
func (p SubdirectoryPath) String() string { return string(p) }

// Validate returns nil if all typed fields of the ModuleRequirement are valid,
// or an error collecting all field-level validation failures.
// GitURL and Version are required; Alias and Path are optional (zero values are valid).
func (r ModuleRequirement) Validate() error {
	var errs []error
	if err := r.GitURL.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := r.Version.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := r.Alias.Validate(); err != nil {
		errs = append(errs, err)
	}
	if err := r.Path.Validate(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// Error implements the error interface for InvalidValidationIssueTypeError.
func (e *InvalidValidationIssueTypeError) Error() string {
	return fmt.Sprintf(
		"invalid validation issue type %q (valid: structure, naming, invowkmod, security, compatibility, invowkfile, command_tree)",
		e.Value,
	)
}

// Unwrap returns the sentinel error for errors.Is() compatibility.
func (e *InvalidValidationIssueTypeError) Unwrap() error {
	return ErrInvalidValidationIssueType
}

// String returns the string representation of the ValidationIssueType.
func (v ValidationIssueType) String() string { return string(v) }

// Validate returns nil if the ValidationIssueType is one of the defined issue types,
// or an error describing the validation failure.
func (v ValidationIssueType) Validate() error {
	switch v {
	case IssueTypeStructure, IssueTypeNaming, IssueTypeInvowkmod, IssueTypeSecurity,
		IssueTypeCompatibility, IssueTypeInvowkfile, IssueTypeCommandTree:
		return nil
	default:
		return &InvalidValidationIssueTypeError{Value: v}
	}
}

// Error implements the error interface for ValidationIssue.
func (v ValidationIssue) Error() string {
	if v.Path != "" {
		return fmt.Sprintf("[%s] %s: %s", v.Type, v.Path, v.Message)
	}
	return fmt.Sprintf("[%s] %s", v.Type, v.Message)
}

// AddIssue adds a validation issue to the result.
// Panics if issueType is not a valid ValidationIssueType — all callers
// pass package-level constants, so an invalid value is a programming error.
func (r *ValidationResult) AddIssue(issueType ValidationIssueType, message, path string) {
	if err := issueType.Validate(); err != nil {
		panic(fmt.Sprintf("AddIssue: %v", err))
	}
	r.Issues = append(r.Issues, ValidationIssue{
		Type:    issueType,
		Message: message,
		Path:    path,
	})
	r.Valid = false
}

// Name returns the module identifier from metadata.
// This is the value of the 'module' field in invowkmod.cue.
func (m *Module) Name() ModuleID {
	if m.Metadata == nil {
		return ""
	}
	return m.Metadata.Module
}

// InvowkmodPath returns the absolute path to invowkmod.cue for this module.
func (m *Module) InvowkmodPath() types.FilesystemPath {
	return types.FilesystemPath(filepath.Join(string(m.Path), "invowkmod.cue")) //goplint:ignore -- derived from validated Module.Path
}

// InvowkfilePath returns the absolute path to invowkfile.cue for this module.
// Returns empty FilesystemPath for library-only modules.
func (m *Module) InvowkfilePath() types.FilesystemPath {
	if m.IsLibraryOnly {
		return ""
	}
	return types.FilesystemPath(filepath.Join(string(m.Path), "invowkfile.cue")) //goplint:ignore -- derived from validated Module.Path
}

// ResolveScriptPath resolves a script path relative to the module root.
// Script paths in modules should use forward slashes for cross-platform compatibility.
// This function converts the cross-platform path to the native format.
func (m *Module) ResolveScriptPath(scriptPath types.FilesystemPath) types.FilesystemPath {
	// Convert forward slashes to native path separator
	nativePath := filepath.FromSlash(string(scriptPath))

	// If already absolute, return as-is
	if filepath.IsAbs(nativePath) {
		return types.FilesystemPath(nativePath) //goplint:ignore -- OS path from filepath.FromSlash
	}

	// Resolve relative to module root
	return types.FilesystemPath(filepath.Join(string(m.Path), nativePath)) //goplint:ignore -- derived from validated Module.Path
}

// ValidateScriptPath checks if a script path is valid for this module.
// Returns an error if the path is invalid (e.g., escapes module directory, is a symlink).
func (m *Module) ValidateScriptPath(scriptPath types.FilesystemPath) error {
	if scriptPath == "" {
		return fmt.Errorf("script path cannot be empty")
	}

	// Convert to native path
	nativePath := filepath.FromSlash(string(scriptPath))

	// Absolute paths are not allowed in modules
	if filepath.IsAbs(nativePath) {
		return fmt.Errorf("absolute paths are not allowed in modules; use paths relative to module root")
	}

	// Resolve the full path
	fullPath := filepath.Join(string(m.Path), nativePath)

	// Ensure the resolved path is within the module (prevent directory traversal)
	relPath, err := filepath.Rel(string(m.Path), fullPath)
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
func (m *Module) ContainsPath(path types.FilesystemPath) bool {
	absPath, err := filepath.Abs(string(path))
	if err != nil {
		return false
	}

	relPath, err := filepath.Rel(string(m.Path), absPath)
	if err != nil {
		return false
	}

	return !strings.HasPrefix(relPath, "..")
}

// GetInvowkfileDir returns the directory containing the invowkfile.
// For modules, this is always the module root.
func (m *Module) GetInvowkfileDir() types.FilesystemPath {
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
	moduleRealPath, err := filepath.EvalSymlinks(string(m.Path))
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
// directRequirements should be the requires list from the module's invowkmod.cue
func NewCommandScope(moduleID ModuleID, globalModuleIDs []ModuleID, directRequirements []ModuleRequirement) *CommandScope {
	scope := &CommandScope{
		ModuleID:      moduleID,
		GlobalModules: make(map[ModuleID]bool),
		DirectDeps:    make(map[ModuleID]bool),
	}

	for _, id := range globalModuleIDs {
		scope.GlobalModules[id] = true
	}

	for _, req := range directRequirements {
		// The direct dependency namespace uses either alias or the resolved module ID
		if req.Alias != "" {
			scope.DirectDeps[ModuleID(string(req.Alias))] = true
		}
		// Note: The actual resolved module ID will be added during resolution
	}

	return scope
}

// CanCall checks if a command can call another command based on scope rules.
// Returns true if allowed, false with reason if not.
func (s *CommandScope) CanCall(targetCmd string) (allowed bool, reason string) {
	// Extract module prefix from command name (format: "module.name cmdname" or "module.name@version cmdname")
	targetModule := ModuleID(ExtractModuleFromCommand(targetCmd)) //goplint:ignore -- used only for equality comparison

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
			"  - Commands from direct dependencies declared in invowkmod.cue:requires\n"+
			"  Add '%s' to your invowkmod.cue requires list to use its commands",
		s.ModuleID, targetCmd, targetModule, s.ModuleID, targetModule)
}

// AddDirectDep adds a resolved direct dependency to the scope.
// This is called during resolution when we know the actual module ID.
func (s *CommandScope) AddDirectDep(moduleID ModuleID) {
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

// ParseInvowkmod reads and parses module metadata from invowkmod.cue at the given path.
func ParseInvowkmod(path types.FilesystemPath) (*Invowkmod, error) {
	data, err := os.ReadFile(string(path))
	if err != nil {
		return nil, fmt.Errorf("failed to read invowkmod at %s: %w", path, err)
	}

	return ParseInvowkmodBytes(data, path)
}

// ParseInvowkmodBytes parses module metadata content from bytes.
// Uses cueutil.ParseAndDecodeString for the 3-step CUE parsing flow:
// compile schema → compile user data → validate and decode.
func ParseInvowkmodBytes(data []byte, path types.FilesystemPath) (*Invowkmod, error) {
	result, err := cueutil.ParseAndDecodeString[Invowkmod](
		invowkmodSchema,
		data,
		"#Invowkmod",
		cueutil.WithFilename(string(path)),
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
			if err := req.Path.Validate(); err != nil {
				return nil, fmt.Errorf("requires[%d].path: %w in invowkmod at %s", i, err, path)
			}
		}
	}

	return meta, nil
}

// ParseModuleMetadataOnly reads and parses only the module metadata (invowkmod.cue) from a module directory.
// This is useful when you only need module identity and dependencies, not commands.
// Returns ErrInvowkmodNotFound if invowkmod.cue doesn't exist.
func ParseModuleMetadataOnly(modulePath types.FilesystemPath) (*Invowkmod, error) {
	invowkmodPath := filepath.Join(string(modulePath), "invowkmod.cue")
	if _, err := os.Stat(invowkmodPath); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrInvowkmodNotFound
		}
		return nil, fmt.Errorf("failed to check invowkmod at %s: %w", invowkmodPath, err)
	}
	return ParseInvowkmod(types.FilesystemPath(invowkmodPath)) //goplint:ignore -- os.Stat confirmed path exists
}

// HasInvowkfile checks if a module directory contains an invowkfile.cue.
func HasInvowkfile(modulePath types.FilesystemPath) bool {
	invowkfilePath := filepath.Join(string(modulePath), "invowkfile.cue")
	_, err := os.Stat(invowkfilePath)
	return err == nil
}

// InvowkfilePath returns the path to invowkfile.cue in a module directory.
func InvowkfilePath(modulePath types.FilesystemPath) types.FilesystemPath {
	return types.FilesystemPath(filepath.Join(string(modulePath), "invowkfile.cue")) //goplint:ignore -- derived from validated modulePath
}

// InvowkmodPath returns the path to invowkmod.cue in a module directory.
//
//nolint:revive // Name is intentional for consistency with Module.InvowkmodPath field/method
func InvowkmodPath(modulePath types.FilesystemPath) types.FilesystemPath {
	return types.FilesystemPath(filepath.Join(string(modulePath), "invowkmod.cue")) //goplint:ignore -- derived from validated modulePath
}
