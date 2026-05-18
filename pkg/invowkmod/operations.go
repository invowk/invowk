// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/pkg/types"
)

type (
	//goplint:validate-all
	//
	// CreateOptions contains options for creating a new module.
	CreateOptions struct {
		// Name is the module name (e.g., "com.example.mytools")
		Name ModuleDirectoryName
		// ParentDir is the directory where the module will be created
		ParentDir types.FilesystemPath
		// Module is the module identifier for the invowkfile (defaults to Name if empty)
		Module ModuleID
		// Description is an optional description for the invowkfile
		Description types.DescriptionText
		// CreateScriptsDir creates a scripts/ subdirectory if true
		CreateScriptsDir bool
	}
)

// IsModule checks if the given path is a valid invowk module directory.
// This is a quick check that only verifies the folder name format and existence.
// For full validation, use Validate().
func IsModule(path types.FilesystemPath) bool {
	pathStr := string(path)

	// Check if the path ends with .invowkmod
	base := filepath.Base(pathStr)
	if !strings.HasSuffix(base, ModuleSuffix) {
		return false
	}

	// Check if the prefix is valid
	prefix := strings.TrimSuffix(base, ModuleSuffix)
	if err := ModuleDirectoryName(prefix).Validate(); err != nil {
		return false
	}

	// Check if it's a real directory (not a symlink). os.Lstat does NOT follow
	// symlinks, preventing symlinked directories from passing module discovery.
	info, err := os.Lstat(pathStr)
	if err != nil {
		return false
	}

	// Reject symlinks: a symlink to a directory should not be treated as a module.
	if info.Mode()&os.ModeSymlink != 0 {
		return false
	}

	return info.IsDir()
}

// ParseModuleName extracts and validates the module name from a folder name.
// The folder name must end with .invowkmod and have a valid prefix.
// Returns the module name (without suffix) or an error if invalid.
func ParseModuleName(folderName string) (ModuleDirectoryName, error) {
	// Must end with .invowkmod
	if !strings.HasSuffix(folderName, ModuleSuffix) {
		return "", fmt.Errorf("folder name must end with '%s'", ModuleSuffix)
	}

	// Extract prefix
	prefix := strings.TrimSuffix(folderName, ModuleSuffix)
	if prefix == "" {
		return "", fmt.Errorf("module name cannot be empty (folder name cannot be just '%s')", ModuleSuffix)
	}

	// Must not start with a dot (hidden folder)
	if strings.HasPrefix(prefix, ".") {
		return "", errors.New("module name cannot start with a dot (hidden folders not allowed)")
	}

	// Validate prefix format
	directoryName := ModuleDirectoryName(prefix)
	if err := directoryName.Validate(); err != nil {
		return "", fmt.Errorf("module name '%s' is invalid: must start with a letter, contain only alphanumeric characters, with optional dot-separated segments (e.g., 'mycommands', 'com.example.utils')", prefix)
	}

	return directoryName, nil
}

// CanonicalModuleDirectoryName returns the local directory basename for moduleID.
// The returned name includes the standard .invowkmod suffix and is suitable for
// cache and vendor materialization.
func CanonicalModuleDirectoryName(moduleID ModuleID) (ModuleScaffoldDirectoryName, error) {
	if err := moduleID.Validate(); err != nil {
		return "", err
	}
	if strings.HasSuffix(moduleID.String(), ModuleSuffix) {
		return "", &InvalidModuleIDError{Value: moduleID}
	}
	dirName := ModuleScaffoldDirectoryName(moduleID.String() + ModuleSuffix)
	if err := dirName.Validate(); err != nil {
		return "", err
	}
	return dirName, nil
}

// ValidateName checks if a module name is valid.
// Returns nil if valid, or an error describing the problem.
func ValidateName(name ModuleDirectoryName) error {
	nameStr := string(name)

	if nameStr == "" {
		return errors.New("module name cannot be empty")
	}

	if strings.HasPrefix(nameStr, ".") {
		return errors.New("module name cannot start with a dot")
	}

	if err := name.Validate(); err != nil {
		return fmt.Errorf("module name '%s' is invalid: must start with a letter, contain only alphanumeric characters, with optional dot-separated segments (e.g., 'mycommands', 'com.example.utils')", nameStr)
	}

	return nil
}
