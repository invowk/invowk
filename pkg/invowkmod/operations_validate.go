// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/pkg/platform"
	"github.com/invowk/invowk/pkg/types"
)

// Validate performs comprehensive validation of a module at the given path.
// Returns a ValidationResult with all issues found, or an error if the path
// cannot be accessed.
func Validate(modulePath types.FilesystemPath) (*ValidationResult, error) {
	// Convert to absolute path
	absPath, err := filepath.Abs(string(modulePath))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	result := &ValidationResult{
		Valid:      true,
		ModulePath: types.FilesystemPath(absPath),
		Issues:     []ValidationIssue{},
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			result.AddIssue(IssueTypeStructure, "path does not exist", "")
			return result, nil
		}
		return nil, fmt.Errorf("failed to stat path: %w", err)
	}

	// Check if it's a directory
	if !info.IsDir() {
		result.AddIssue(IssueTypeStructure, "path is not a directory", "")
		return result, nil
	}

	// Validate folder name and extract module name
	base := filepath.Base(absPath)
	moduleName, err := ParseModuleName(base)
	if err != nil {
		result.AddIssue(IssueTypeNaming, err.Error(), "")
	} else {
		result.ModuleName = moduleName

		// Check for reserved module name "invowkfile" (FR-015)
		// This name is reserved for the canonical namespace system where @invowkfile
		// refers to the root invowkfile.cue source
		if string(moduleName) == "invowkfile" {
			result.AddIssue(IssueTypeNaming, "module name 'invowkfile' is reserved for the root invowkfile source", "")
		}
	}

	// Check for invowkmod.cue (required)
	invowkmodPath := filepath.Join(absPath, "invowkmod.cue")
	invowkmodInfo, err := os.Stat(invowkmodPath)
	switch {
	case err != nil && os.IsNotExist(err):
		result.AddIssue(IssueTypeStructure, "missing required invowkmod.cue", "")
	case err != nil:
		result.AddIssue(IssueTypeStructure, fmt.Sprintf("cannot access invowkmod.cue: %v", err), "")
	case invowkmodInfo.IsDir():
		result.AddIssue(IssueTypeStructure, "invowkmod.cue must be a file, not a directory", "")
	default:
		result.InvowkmodPath = types.FilesystemPath(invowkmodPath)

		// Parse invowkmod.cue and validate module field matches folder name
		if result.ModuleName != "" {
			meta, parseErr := ParseInvowkmod(types.FilesystemPath(invowkmodPath))
			if parseErr != nil {
				result.AddIssue(IssueTypeInvowkmod, fmt.Sprintf("failed to parse invowkmod.cue: %v", parseErr), "invowkmod.cue")
			} else if string(meta.Module) != string(result.ModuleName) {
				result.AddIssue(IssueTypeNaming, fmt.Sprintf(
					"module field '%s' in invowkmod.cue must match folder name '%s'",
					meta.Module, result.ModuleName), "invowkmod.cue")
			}
		}
	}

	// Check for invowkfile.cue (optional - may be a library-only module)
	invowkfilePath := filepath.Join(absPath, "invowkfile.cue")
	invowkfileInfo, err := os.Stat(invowkfilePath)
	switch {
	case err != nil && os.IsNotExist(err):
		// Library-only module - no commands
		result.IsLibraryOnly = true
	case err != nil:
		result.AddIssue(IssueTypeStructure, fmt.Sprintf("cannot access invowkfile.cue: %v", err), "")
	case invowkfileInfo.IsDir():
		result.AddIssue(IssueTypeStructure, "invowkfile.cue must be a file, not a directory", "")
	default:
		result.InvowkfilePath = types.FilesystemPath(invowkfilePath)
	}

	// Check for nested modules and symlinks (security)
	err = filepath.WalkDir(absPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil //nolint:nilerr // Intentionally skip errors to continue walking
		}

		// Skip the root directory itself
		if path == absPath {
			return nil
		}

		// Skip the vendored modules directory (invowk_modules/) - nested modules are allowed there
		if d.IsDir() && d.Name() == VendoredModulesDir {
			return filepath.SkipDir
		}

		// Check for symlinks (security issue - could point outside module)
		if d.Type()&os.ModeSymlink != 0 {
			relPath, _ := filepath.Rel(absPath, path)
			// Check if symlink points outside the module
			linkTarget, readErr := os.Readlink(path)
			if readErr != nil {
				result.AddIssue(IssueTypeSecurity, "cannot read symlink target", relPath)
			} else {
				// Resolve the symlink target relative to its location
				var resolvedTarget string
				if filepath.IsAbs(linkTarget) {
					resolvedTarget = linkTarget
				} else {
					resolvedTarget = filepath.Join(filepath.Dir(path), linkTarget)
				}
				// Clean and resolve to check if it escapes
				resolvedTarget = filepath.Clean(resolvedTarget)
				relToRoot, relErr := filepath.Rel(absPath, resolvedTarget)
				if relErr != nil || strings.HasPrefix(relToRoot, "..") {
					result.AddIssue(IssueTypeSecurity, fmt.Sprintf("symlink points outside module directory (target: %s)", linkTarget), relPath)
				} else {
					// Even internal symlinks are a potential security concern during archive extraction
					result.AddIssue(IssueTypeSecurity, "symlinks are not allowed in modules (security risk during extraction)", relPath)
				}
			}
		}

		// Check if any other subdirectory is a module
		if d.IsDir() && strings.HasSuffix(d.Name(), ModuleSuffix) {
			relPath, _ := filepath.Rel(absPath, path)
			result.AddIssue(IssueTypeStructure, "nested modules are not allowed (except in invowk_modules/)", relPath)
		}

		// Check for Windows reserved filenames (cross-platform compatibility)
		if platform.IsWindowsReservedName(d.Name()) {
			relPath, _ := filepath.Rel(absPath, path)
			result.AddIssue(IssueTypeCompatibility, fmt.Sprintf("filename '%s' is reserved on Windows", d.Name()), relPath)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk module directory: %w", err)
	}

	return result, nil
}

// Load loads and validates a module at the given path.
// Returns a Module (operational wrapper) if valid, or an error with validation details.
// Note: This loads only metadata (invowkmod.cue), not commands (invowkfile.cue).
// To load commands as well, use pkg/invowkfile.ParseModule().
func Load(modulePath types.FilesystemPath) (*Module, error) {
	result, err := Validate(modulePath)
	if err != nil {
		return nil, err
	}

	if !result.Valid {
		// Collect all issues into error message
		var msgs []string
		for _, issue := range result.Issues {
			msgs = append(msgs, issue.Error())
		}
		return nil, fmt.Errorf("invalid module: %s", strings.Join(msgs, "; "))
	}

	// Parse the metadata
	var metadata *Invowkmod
	if result.InvowkmodPath != "" {
		metadata, err = ParseInvowkmod(result.InvowkmodPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse module metadata: %w", err)
		}
	}

	return &Module{
		Metadata:      metadata,
		Path:          result.ModulePath,
		IsLibraryOnly: result.IsLibraryOnly,
	}, nil
}
