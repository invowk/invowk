// SPDX-License-Identifier: EPL-2.0

package invkpack

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

//go:embed invkpack_schema.cue
var invkpackSchema string

// MaxPathLength is the maximum allowed length for file paths.
const MaxPathLength = 4096

// PackSuffix is the standard suffix for invowk pack directories.
const PackSuffix = ".invkpack"

// VendoredPacksDir is the directory name for vendored pack dependencies.
const VendoredPacksDir = "invk_packs"

// ValidationIssue represents a single validation problem in a pack.
type ValidationIssue struct {
	// Type categorizes the issue (e.g., "structure", "naming", "invkfile")
	Type string
	// Message describes the specific problem
	Message string
	// Path is the relative path within the pack where the issue was found (optional)
	Path string
}

// Error implements the error interface for ValidationIssue.
func (v ValidationIssue) Error() string {
	if v.Path != "" {
		return fmt.Sprintf("[%s] %s: %s", v.Type, v.Path, v.Message)
	}
	return fmt.Sprintf("[%s] %s", v.Type, v.Message)
}

// ValidationResult contains the result of pack validation.
type ValidationResult struct {
	// Valid is true if the pack passed all validation checks
	Valid bool
	// PackPath is the absolute path to the validated pack
	PackPath string
	// PackName is the extracted name from the folder (without .invkpack suffix)
	PackName string
	// InvkpackPath is the path to the invkpack.cue within the pack (required)
	InvkpackPath string
	// InvkfilePath is the path to the invkfile.cue within the pack (optional for library-only packs)
	InvkfilePath string
	// IsLibraryOnly is true if the pack has no invkfile.cue
	IsLibraryOnly bool
	// Issues contains all validation problems found
	Issues []ValidationIssue
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

// Pack represents a loaded invowk pack, ready for use.
// This is the unified type combining filesystem structure with parsed content.
type Pack struct {
	// Metadata is the parsed invkpack.cue content (always present after Load())
	Metadata *Invkpack

	// Commands is the parsed invkfile.cue content (nil for library-only packs)
	// Type is any to avoid circular imports with pkg/invkfile.
	// The actual type is *invkfile.Invkfile.
	Commands any

	// Path is the absolute filesystem path to the pack directory
	Path string

	// IsLibraryOnly is true if the pack has no invkfile.cue
	IsLibraryOnly bool
}

// Name returns the pack identifier from metadata.
// This is the value of the 'pack' field in invkpack.cue.
func (p *Pack) Name() string {
	if p.Metadata == nil {
		return ""
	}
	return p.Metadata.Pack
}

// InvkpackPath returns the absolute path to invkpack.cue for this pack.
func (p *Pack) InvkpackPath() string {
	return filepath.Join(p.Path, "invkpack.cue")
}

// InvkfilePath returns the absolute path to invkfile.cue for this pack.
// Returns empty string for library-only packs.
func (p *Pack) InvkfilePath() string {
	if p.IsLibraryOnly {
		return ""
	}
	return filepath.Join(p.Path, "invkfile.cue")
}

// ResolveScriptPath resolves a script path relative to the pack root.
// Script paths in packs should use forward slashes for cross-platform compatibility.
// This function converts the cross-platform path to the native format.
func (p *Pack) ResolveScriptPath(scriptPath string) string {
	// Convert forward slashes to native path separator
	nativePath := filepath.FromSlash(scriptPath)

	// If already absolute, return as-is
	if filepath.IsAbs(nativePath) {
		return nativePath
	}

	// Resolve relative to pack root
	return filepath.Join(p.Path, nativePath)
}

// ValidateScriptPath checks if a script path is valid for this pack.
// Returns an error if the path is invalid (e.g., escapes pack directory, is a symlink).
func (p *Pack) ValidateScriptPath(scriptPath string) error {
	if scriptPath == "" {
		return fmt.Errorf("script path cannot be empty")
	}

	// Convert to native path
	nativePath := filepath.FromSlash(scriptPath)

	// Absolute paths are not allowed in packs
	if filepath.IsAbs(nativePath) {
		return fmt.Errorf("absolute paths are not allowed in packs; use paths relative to pack root")
	}

	// Resolve the full path
	fullPath := filepath.Join(p.Path, nativePath)

	// Ensure the resolved path is within the pack (prevent directory traversal)
	relPath, err := filepath.Rel(p.Path, fullPath)
	if err != nil {
		return fmt.Errorf("failed to resolve relative path: %w", err)
	}

	// Check for path escaping (e.g., "../something")
	if strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("script path '%s' escapes the pack directory", scriptPath)
	}

	// Check if the path or any parent is a symlink
	if err := p.checkSymlinkSafety(fullPath); err != nil {
		return err
	}

	return nil
}

// checkSymlinkSafety verifies that a path doesn't contain symlinks that could escape the pack.
func (p *Pack) checkSymlinkSafety(path string) error {
	// Get the real path by resolving all symlinks
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		// If the file doesn't exist, that's fine - it'll be caught elsewhere
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot resolve symlinks in path: %w", err)
	}

	// Ensure the real path is still within the pack
	packRealPath, err := filepath.EvalSymlinks(p.Path)
	if err != nil {
		return fmt.Errorf("cannot resolve pack path: %w", err)
	}

	relPath, err := filepath.Rel(packRealPath, realPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("path resolves to location outside pack directory (symlink escape)")
	}

	return nil
}

// ContainsPath checks if the given path is inside this pack.
func (p *Pack) ContainsPath(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	relPath, err := filepath.Rel(p.Path, absPath)
	if err != nil {
		return false
	}

	return !strings.HasPrefix(relPath, "..")
}

// GetInvkfileDir returns the directory containing the invkfile.
// For packs, this is always the pack root.
func (p *Pack) GetInvkfileDir() string {
	return p.Path
}

// PackRequirement represents a dependency on another pack from a Git repository.
type PackRequirement struct {
	// GitURL is the Git repository URL (HTTPS or SSH format).
	// Examples: "https://github.com/user/repo.git", "git@github.com:user/repo.git"
	GitURL string `json:"git_url"`
	// Version is the semver constraint for version selection.
	// Examples: "^1.2.0", "~1.2.0", ">=1.0.0 <2.0.0", "1.2.3"
	Version string `json:"version"`
	// Alias overrides the default namespace for imported commands (optional).
	// If not set, the namespace is: <pack>@<resolved-version>
	Alias string `json:"alias,omitempty"`
	// Path specifies a subdirectory containing the pack (optional).
	// Used for monorepos with multiple packs.
	Path string `json:"path,omitempty"`
}

// Invkpack represents pack metadata from invkpack.cue.
// This is analogous to Go's go.mod file - it contains pack identity and dependencies.
// Command definitions remain in invkfile.cue (separate file).
type Invkpack struct {
	// Pack is a MANDATORY identifier for this pack.
	// Acts as pack identity and command namespace prefix.
	// Must start with a letter, contain only alphanumeric characters, with optional
	// dot-separated segments. RDNS format recommended (e.g., "io.invowk.sample", "com.example.mytools")
	// IMPORTANT: The pack value MUST match the folder name prefix (before .invkpack)
	Pack string `json:"pack"`
	// Version specifies the pack schema version (optional but recommended).
	// Current version: "1.0"
	Version string `json:"version,omitempty"`
	// Description provides a summary of this pack's purpose (optional).
	Description string `json:"description,omitempty"`
	// Requires declares dependencies on other packs from Git repositories (optional).
	// Dependencies are resolved at pack level.
	// All required packs are loaded and their commands made available.
	// IMPORTANT: Commands in this pack can ONLY call:
	//   1. Commands from globally installed packs (~/.invowk/packs/)
	//   2. Commands from packs declared directly in THIS requires list
	// Commands CANNOT call transitive dependencies (dependencies of dependencies).
	Requires []PackRequirement `json:"requires,omitempty"`
	// FilePath stores the path where this invkpack.cue was loaded from (not in CUE)
	FilePath string `json:"-"`
}

// CommandScope defines what commands a pack can access.
// Commands in a pack can ONLY call:
//  1. Commands from the same pack
//  2. Commands from globally installed packs (~/.invowk/packs/)
//  3. Commands from first-level requirements (direct dependencies in invkpack.cue:requires)
//
// Commands CANNOT call transitive dependencies (dependencies of dependencies).
type CommandScope struct {
	// PackID is the pack identifier that owns this scope
	PackID string
	// GlobalPacks are commands from globally installed packs (always accessible)
	GlobalPacks map[string]bool
	// DirectDeps are pack IDs from first-level requirements (from invkpack.cue:requires)
	DirectDeps map[string]bool
}

// NewCommandScope creates a CommandScope for a parsed pack.
// globalPackIDs should contain pack IDs from ~/.invowk/packs/
// directRequirements should be the requires list from the pack's invkpack.cue
func NewCommandScope(packID string, globalPackIDs []string, directRequirements []PackRequirement) *CommandScope {
	scope := &CommandScope{
		PackID:      packID,
		GlobalPacks: make(map[string]bool),
		DirectDeps:  make(map[string]bool),
	}

	for _, id := range globalPackIDs {
		scope.GlobalPacks[id] = true
	}

	for _, req := range directRequirements {
		// The direct dependency namespace uses either alias or the resolved pack ID
		if req.Alias != "" {
			scope.DirectDeps[req.Alias] = true
		}
		// Note: The actual resolved pack ID will be added during resolution
	}

	return scope
}

// CanCall checks if a command can call another command based on scope rules.
// Returns true if allowed, false with reason if not.
func (s *CommandScope) CanCall(targetCmd string) (allowed bool, reason string) {
	// Extract pack prefix from command name (format: "pack.name cmdname" or "pack.name@version cmdname")
	targetPack := ExtractPackFromCommand(targetCmd)

	// If no pack prefix, it's a local command (always allowed)
	if targetPack == "" {
		return true, ""
	}

	// Check if target is from same pack
	if targetPack == s.PackID {
		return true, ""
	}

	// Check if target is in global packs
	if s.GlobalPacks[targetPack] {
		return true, ""
	}

	// Check if target is in direct dependencies
	if s.DirectDeps[targetPack] {
		return true, ""
	}

	return false, fmt.Sprintf(
		"command from pack '%s' cannot call '%s': pack '%s' is not accessible\n"+
			"  Commands can only call:\n"+
			"  - Commands from the same pack (%s)\n"+
			"  - Commands from globally installed packs (~/.invowk/packs/)\n"+
			"  - Commands from direct dependencies declared in invkpack.cue:requires\n"+
			"  Add '%s' to your invkpack.cue requires list to use its commands",
		s.PackID, targetCmd, targetPack, s.PackID, targetPack)
}

// AddDirectDep adds a resolved direct dependency to the scope.
// This is called during resolution when we know the actual pack ID.
func (s *CommandScope) AddDirectDep(packID string) {
	s.DirectDeps[packID] = true
}

// ExtractPackFromCommand extracts the pack prefix from a fully qualified command name.
// Returns empty string if no pack prefix found.
// Examples:
//   - "io.invowk.sample hello" -> "io.invowk.sample"
//   - "utils@1.2.3 build" -> "utils@1.2.3"
//   - "build" -> ""
func ExtractPackFromCommand(cmd string) string {
	// Command format: "pack cmdname" where pack may contain dots and @version
	parts := strings.SplitN(cmd, " ", 2)
	if len(parts) < 2 {
		// No space means it's either a local command or just a pack with no command
		return ""
	}
	return parts[0]
}

// ParseInvkpack reads and parses pack metadata from invkpack.cue at the given path.
func ParseInvkpack(path string) (*Invkpack, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read invkpack at %s: %w", path, err)
	}

	return ParseInvkpackBytes(data, path)
}

// ParseInvkpackBytes parses pack metadata content from bytes.
func ParseInvkpackBytes(data []byte, path string) (*Invkpack, error) {
	ctx := cuecontext.New()

	// Compile the schema
	schemaValue := ctx.CompileString(invkpackSchema)
	if schemaValue.Err() != nil {
		return nil, fmt.Errorf("internal error: failed to compile invkpack schema: %w", schemaValue.Err())
	}

	// Compile the user's invkpack file
	userValue := ctx.CompileBytes(data, cue.Filename(path))
	if userValue.Err() != nil {
		return nil, fmt.Errorf("failed to parse invkpack at %s: %w", path, userValue.Err())
	}

	// Unify with schema to validate
	schema := schemaValue.LookupPath(cue.ParsePath("#Invkpack"))
	unified := schema.Unify(userValue)
	if err := unified.Validate(cue.Concrete(true)); err != nil {
		return nil, fmt.Errorf("invkpack validation failed at %s: %w", path, err)
	}

	// Decode into struct
	var meta Invkpack
	if err := unified.Decode(&meta); err != nil {
		return nil, fmt.Errorf("failed to decode invkpack at %s: %w", path, err)
	}

	meta.FilePath = path

	// Validate pack requirement paths for security
	for i, req := range meta.Requires {
		if req.Path != "" {
			if len(req.Path) > MaxPathLength {
				return nil, fmt.Errorf("requires[%d].path: too long (%d chars, max %d) in invkpack at %s", i, len(req.Path), MaxPathLength, path)
			}
			if strings.ContainsRune(req.Path, '\x00') {
				return nil, fmt.Errorf("requires[%d].path: contains null byte in invkpack at %s", i, path)
			}
			cleanPath := filepath.Clean(req.Path)
			if strings.HasPrefix(cleanPath, "..") || filepath.IsAbs(cleanPath) {
				return nil, fmt.Errorf("requires[%d].path: path traversal or absolute paths not allowed in invkpack at %s", i, path)
			}
		}
	}

	return &meta, nil
}

// ParsePackMetadataOnly reads and parses only the pack metadata (invkpack.cue) from a pack directory.
// This is useful when you only need pack identity and dependencies, not commands.
// Returns nil if invkpack.cue doesn't exist.
func ParsePackMetadataOnly(packPath string) (*Invkpack, error) {
	invkpackPath := filepath.Join(packPath, "invkpack.cue")
	if _, err := os.Stat(invkpackPath); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to check invkpack at %s: %w", invkpackPath, err)
	}
	return ParseInvkpack(invkpackPath)
}

// HasInvkfile checks if a pack directory contains an invkfile.cue.
func HasInvkfile(packPath string) bool {
	invkfilePath := filepath.Join(packPath, "invkfile.cue")
	_, err := os.Stat(invkfilePath)
	return err == nil
}

// InvkfilePath returns the path to invkfile.cue in a pack directory.
func InvkfilePath(packPath string) string {
	return filepath.Join(packPath, "invkfile.cue")
}

// InvkpackPath returns the path to invkpack.cue in a pack directory.
//
//nolint:revive // InvkpackPath is more descriptive than Path for external callers
func InvkpackPath(packPath string) string {
	return filepath.Join(packPath, "invkpack.cue")
}
