// SPDX-License-Identifier: MPL-2.0

package invkmod

import (
	"fmt"
	"invowk-cli/pkg/platform"
	"os"
	"path/filepath"
	"strings"
)

// Validate performs comprehensive validation of a module at the given path.
// Returns a ValidationResult with all issues found, or an error if the path
// cannot be accessed.
func Validate(modulePath string) (*ValidationResult, error) {
	// Convert to absolute path
	absPath, err := filepath.Abs(modulePath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	result := &ValidationResult{
		Valid:      true,
		ModulePath: absPath,
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

	// Validate folder name and extract module name
	base := filepath.Base(absPath)
	moduleName, err := ParseModuleName(base)
	if err != nil {
		result.AddIssue("naming", err.Error(), "")
	} else {
		result.ModuleName = moduleName

		// Check for reserved module name "invkfile" (FR-015)
		// This name is reserved for the canonical namespace system where @invkfile
		// refers to the root invkfile.cue source
		if moduleName == "invkfile" {
			result.AddIssue("naming", "module name 'invkfile' is reserved for the root invkfile source", "")
		}
	}

	// Check for invkmod.cue (required)
	invkmodPath := filepath.Join(absPath, "invkmod.cue")
	invkmodInfo, err := os.Stat(invkmodPath)
	switch {
	case err != nil && os.IsNotExist(err):
		result.AddIssue("structure", "missing required invkmod.cue", "")
	case err != nil:
		result.AddIssue("structure", fmt.Sprintf("cannot access invkmod.cue: %v", err), "")
	case invkmodInfo.IsDir():
		result.AddIssue("structure", "invkmod.cue must be a file, not a directory", "")
	default:
		result.InvkmodPath = invkmodPath

		// Parse invkmod.cue and validate module field matches folder name
		if result.ModuleName != "" {
			meta, parseErr := ParseInvkmod(invkmodPath)
			if parseErr != nil {
				result.AddIssue("invkmod", fmt.Sprintf("failed to parse invkmod.cue: %v", parseErr), "invkmod.cue")
			} else if meta.Module != result.ModuleName {
				result.AddIssue("naming", fmt.Sprintf(
					"module field '%s' in invkmod.cue must match folder name '%s'",
					meta.Module, result.ModuleName), "invkmod.cue")
			}
		}
	}

	// Check for invkfile.cue (optional - may be a library-only module)
	invkfilePath := filepath.Join(absPath, "invkfile.cue")
	invkfileInfo, err := os.Stat(invkfilePath)
	switch {
	case err != nil && os.IsNotExist(err):
		// Library-only module - no commands
		result.IsLibraryOnly = true
	case err != nil:
		result.AddIssue("structure", fmt.Sprintf("cannot access invkfile.cue: %v", err), "")
	case invkfileInfo.IsDir():
		result.AddIssue("structure", "invkfile.cue must be a file, not a directory", "")
	default:
		result.InvkfilePath = invkfilePath
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

		// Skip the vendored modules directory (invk_modules/) - nested modules are allowed there
		if d.IsDir() && d.Name() == VendoredModulesDir {
			return filepath.SkipDir
		}

		// Check for symlinks (security issue - could point outside module)
		if d.Type()&os.ModeSymlink != 0 {
			relPath, _ := filepath.Rel(absPath, path)
			// Check if symlink points outside the module
			linkTarget, readErr := os.Readlink(path)
			if readErr != nil {
				result.AddIssue("security", "cannot read symlink target", relPath)
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
					result.AddIssue("security", fmt.Sprintf("symlink points outside module directory (target: %s)", linkTarget), relPath)
				} else {
					// Even internal symlinks are a potential security concern during archive extraction
					result.AddIssue("security", "symlinks are not allowed in modules (security risk during extraction)", relPath)
				}
			}
		}

		// Check if any other subdirectory is a module
		if d.IsDir() && strings.HasSuffix(d.Name(), ModuleSuffix) {
			relPath, _ := filepath.Rel(absPath, path)
			result.AddIssue("structure", "nested modules are not allowed (except in invk_modules/)", relPath)
		}

		// Check for Windows reserved filenames (cross-platform compatibility)
		if platform.IsWindowsReservedName(d.Name()) {
			relPath, _ := filepath.Rel(absPath, path)
			result.AddIssue("compatibility", fmt.Sprintf("filename '%s' is reserved on Windows", d.Name()), relPath)
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
// Note: This loads only metadata (invkmod.cue), not commands (invkfile.cue).
// To load commands as well, use pkg/invkfile.ParseModule().
func Load(modulePath string) (*Module, error) {
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
	var metadata *Invkmod
	if result.InvkmodPath != "" {
		metadata, err = ParseInvkmod(result.InvkmodPath)
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
