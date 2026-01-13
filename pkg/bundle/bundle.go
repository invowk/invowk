// Package bundle provides functionality for working with invowk bundles.
//
// A bundle is a self-contained folder with a ".invowkbundle" suffix that contains
// an invowkfile and optionally script files. Bundles enable portable distribution
// of invowk commands with their associated scripts.
//
// Bundle naming follows these rules:
//   - Folder name must end with ".invowkbundle"
//   - Prefix (before .invowkbundle) must be POSIX-compliant: start with a letter,
//     contain only alphanumeric characters, with optional dot-separated segments
//   - Compatible with RDNS naming conventions (e.g., "com.example.mycommands")
//
// Bundle structure:
//   - Must contain exactly one invowkfile.cue at the root
//   - May contain script files referenced by implementations
//   - Cannot be nested inside other bundles
package bundle

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// BundleSuffix is the standard suffix for invowk bundle directories
const BundleSuffix = ".invowkbundle"

// bundleNameRegex validates the bundle folder name prefix (before .invowkbundle)
// Must start with a letter, contain only alphanumeric chars, with optional dot-separated segments
// Compatible with RDNS naming (e.g., "com.example.mycommands")
var bundleNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*(\.[a-zA-Z][a-zA-Z0-9]*)*$`)

// ValidationIssue represents a single validation problem in a bundle
type ValidationIssue struct {
	// Type categorizes the issue (e.g., "structure", "naming", "invowkfile")
	Type string
	// Message describes the specific problem
	Message string
	// Path is the relative path within the bundle where the issue was found (optional)
	Path string
}

// Error implements the error interface for ValidationIssue
func (v ValidationIssue) Error() string {
	if v.Path != "" {
		return fmt.Sprintf("[%s] %s: %s", v.Type, v.Path, v.Message)
	}
	return fmt.Sprintf("[%s] %s", v.Type, v.Message)
}

// ValidationResult contains the result of bundle validation
type ValidationResult struct {
	// Valid is true if the bundle passed all validation checks
	Valid bool
	// BundlePath is the absolute path to the validated bundle
	BundlePath string
	// BundleName is the extracted name from the folder (without .invowkbundle suffix)
	BundleName string
	// InvowkfilePath is the path to the invowkfile.cue within the bundle
	InvowkfilePath string
	// Issues contains all validation problems found
	Issues []ValidationIssue
}

// AddIssue adds a validation issue to the result
func (r *ValidationResult) AddIssue(issueType, message, path string) {
	r.Issues = append(r.Issues, ValidationIssue{
		Type:    issueType,
		Message: message,
		Path:    path,
	})
	r.Valid = false
}

// Bundle represents a validated invowk bundle
type Bundle struct {
	// Path is the absolute path to the bundle directory
	Path string
	// Name is the bundle name (folder name without .invowkbundle suffix)
	Name string
	// InvowkfilePath is the absolute path to the invowkfile.cue
	InvowkfilePath string
}

// IsBundle checks if the given path is a valid invowk bundle directory.
// This is a quick check that only verifies the folder name format and existence.
// For full validation, use Validate().
func IsBundle(path string) bool {
	// Check if the path ends with .invowkbundle
	base := filepath.Base(path)
	if !strings.HasSuffix(base, BundleSuffix) {
		return false
	}

	// Check if the prefix is valid
	prefix := strings.TrimSuffix(base, BundleSuffix)
	if !bundleNameRegex.MatchString(prefix) {
		return false
	}

	// Check if it's a directory
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	return info.IsDir()
}

// ParseBundleName extracts and validates the bundle name from a folder name.
// The folder name must end with .invowkbundle and have a valid prefix.
// Returns the bundle name (without suffix) or an error if invalid.
func ParseBundleName(folderName string) (string, error) {
	// Must end with .invowkbundle
	if !strings.HasSuffix(folderName, BundleSuffix) {
		return "", fmt.Errorf("folder name must end with '%s'", BundleSuffix)
	}

	// Extract prefix
	prefix := strings.TrimSuffix(folderName, BundleSuffix)
	if prefix == "" {
		return "", fmt.Errorf("bundle name cannot be empty (folder name cannot be just '%s')", BundleSuffix)
	}

	// Must not start with a dot (hidden folder)
	if strings.HasPrefix(prefix, ".") {
		return "", fmt.Errorf("bundle name cannot start with a dot (hidden folders not allowed)")
	}

	// Validate prefix format
	if !bundleNameRegex.MatchString(prefix) {
		return "", fmt.Errorf("bundle name '%s' is invalid: must start with a letter, contain only alphanumeric characters, with optional dot-separated segments (e.g., 'mycommands', 'com.example.utils')", prefix)
	}

	return prefix, nil
}

// Validate performs comprehensive validation of a bundle at the given path.
// Returns a ValidationResult with all issues found, or an error if the path
// cannot be accessed.
func Validate(bundlePath string) (*ValidationResult, error) {
	// Convert to absolute path
	absPath, err := filepath.Abs(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	result := &ValidationResult{
		Valid:      true,
		BundlePath: absPath,
		Issues:     []ValidationIssue{},
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			result.AddIssue("structure", "path does not exist", "")
			return result, nil
		}
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	// Check if it's a directory
	if !info.IsDir() {
		result.AddIssue("structure", "path is not a directory", "")
		return result, nil
	}

	// Validate folder name and extract bundle name
	base := filepath.Base(absPath)
	bundleName, err := ParseBundleName(base)
	if err != nil {
		result.AddIssue("naming", err.Error(), "")
	} else {
		result.BundleName = bundleName
	}

	// Check for invowkfile.cue
	invowkfilePath := filepath.Join(absPath, "invowkfile.cue")
	invowkfileInfo, err := os.Stat(invowkfilePath)
	if err != nil {
		if os.IsNotExist(err) {
			result.AddIssue("structure", "missing required invowkfile.cue", "")
		} else {
			result.AddIssue("structure", fmt.Sprintf("cannot access invowkfile.cue: %v", err), "")
		}
	} else if invowkfileInfo.IsDir() {
		result.AddIssue("structure", "invowkfile.cue must be a file, not a directory", "")
	} else {
		result.InvowkfilePath = invowkfilePath
	}

	// Check for nested bundles (not allowed)
	err = filepath.WalkDir(absPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // Continue walking even on errors
		}

		// Skip the root directory itself
		if path == absPath {
			return nil
		}

		// Check if any subdirectory is a bundle
		if d.IsDir() && strings.HasSuffix(d.Name(), BundleSuffix) {
			relPath, _ := filepath.Rel(absPath, path)
			result.AddIssue("structure", "nested bundles are not allowed", relPath)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk bundle directory: %w", err)
	}

	return result, nil
}

// Load loads and validates a bundle at the given path.
// Returns a Bundle struct if valid, or an error with validation details.
func Load(bundlePath string) (*Bundle, error) {
	result, err := Validate(bundlePath)
	if err != nil {
		return nil, err
	}

	if !result.Valid {
		// Collect all issues into error message
		var msgs []string
		for _, issue := range result.Issues {
			msgs = append(msgs, issue.Error())
		}
		return nil, fmt.Errorf("invalid bundle: %s", strings.Join(msgs, "; "))
	}

	return &Bundle{
		Path:           result.BundlePath,
		Name:           result.BundleName,
		InvowkfilePath: result.InvowkfilePath,
	}, nil
}

// ResolveScriptPath resolves a script path relative to the bundle root.
// Script paths in bundles should use forward slashes for cross-platform compatibility.
// This function converts the cross-platform path to the native format.
func (b *Bundle) ResolveScriptPath(scriptPath string) string {
	// Convert forward slashes to native path separator
	nativePath := filepath.FromSlash(scriptPath)

	// If already absolute, return as-is
	if filepath.IsAbs(nativePath) {
		return nativePath
	}

	// Resolve relative to bundle root
	return filepath.Join(b.Path, nativePath)
}

// ValidateScriptPath checks if a script path is valid for this bundle.
// Returns an error if the path is invalid (e.g., escapes bundle directory).
func (b *Bundle) ValidateScriptPath(scriptPath string) error {
	if scriptPath == "" {
		return fmt.Errorf("script path cannot be empty")
	}

	// Convert to native path
	nativePath := filepath.FromSlash(scriptPath)

	// Absolute paths are not allowed in bundles
	if filepath.IsAbs(nativePath) {
		return fmt.Errorf("absolute paths are not allowed in bundles; use paths relative to bundle root")
	}

	// Resolve the full path
	fullPath := filepath.Join(b.Path, nativePath)

	// Ensure the resolved path is within the bundle (prevent directory traversal)
	relPath, err := filepath.Rel(b.Path, fullPath)
	if err != nil {
		return fmt.Errorf("failed to resolve relative path: %w", err)
	}

	// Check for path escaping (e.g., "../something")
	if strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("script path '%s' escapes the bundle directory", scriptPath)
	}

	return nil
}

// ContainsPath checks if the given path is inside this bundle.
func (b *Bundle) ContainsPath(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	relPath, err := filepath.Rel(b.Path, absPath)
	if err != nil {
		return false
	}

	return !strings.HasPrefix(relPath, "..")
}

// GetInvowkfileDir returns the directory containing the invowkfile.
// For bundles, this is always the bundle root.
func (b *Bundle) GetInvowkfileDir() string {
	return b.Path
}
