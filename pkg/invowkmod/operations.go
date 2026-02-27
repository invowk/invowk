// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/invowk/invowk/pkg/types"
)

// moduleNameRegex validates the module folder name prefix (before .invowkmod)
// Must start with a letter, contain only alphanumeric chars, with optional dot-separated segments
// Compatible with RDNS naming (e.g., "com.example.mycommands")
var moduleNameRegex = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*(\.[a-zA-Z][a-zA-Z0-9]*)*$`)

type (
	// CreateOptions contains options for creating a new module.
	CreateOptions struct {
		// Name is the module name (e.g., "com.example.mytools")
		Name ModuleShortName
		// ParentDir is the directory where the module will be created
		ParentDir types.FilesystemPath
		// Module is the module identifier for the invowkfile (defaults to Name if empty)
		Module ModuleID
		// Description is an optional description for the invowkfile
		Description types.DescriptionText
		// CreateScriptsDir creates a scripts/ subdirectory if true
		CreateScriptsDir bool
	}

	// UnpackOptions contains options for unpacking a module.
	UnpackOptions struct {
		// Source is the path to the ZIP file or URL; intentionally untyped (mixed path/URL).
		Source string
		// DestDir is the destination directory (defaults to current directory)
		DestDir types.FilesystemPath
		// Overwrite allows overwriting an existing module
		Overwrite bool
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
	if !moduleNameRegex.MatchString(prefix) {
		return false
	}

	// Check if it's a directory
	info, err := os.Stat(pathStr)
	if err != nil {
		return false
	}

	return info.IsDir()
}

// ParseModuleName extracts and validates the module name from a folder name.
// The folder name must end with .invowkmod and have a valid prefix.
// Returns the module name (without suffix) or an error if invalid.
func ParseModuleName(folderName string) (ModuleShortName, error) {
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
		return "", fmt.Errorf("module name cannot start with a dot (hidden folders not allowed)")
	}

	// Validate prefix format
	if !moduleNameRegex.MatchString(prefix) {
		return "", fmt.Errorf("module name '%s' is invalid: must start with a letter, contain only alphanumeric characters, with optional dot-separated segments (e.g., 'mycommands', 'com.example.utils')", prefix)
	}

	shortName := ModuleShortName(prefix)
	if err := shortName.Validate(); err != nil {
		return "", fmt.Errorf("module short name: %w", err)
	}
	return shortName, nil
}

// ValidateName checks if a module name is valid.
// Returns nil if valid, or an error describing the problem.
func ValidateName(name ModuleShortName) error {
	nameStr := string(name)

	if nameStr == "" {
		return fmt.Errorf("module name cannot be empty")
	}

	if strings.HasPrefix(nameStr, ".") {
		return fmt.Errorf("module name cannot start with a dot")
	}

	if !moduleNameRegex.MatchString(nameStr) {
		return fmt.Errorf("module name '%s' is invalid: must start with a letter, contain only alphanumeric characters, with optional dot-separated segments (e.g., 'mycommands', 'com.example.utils')", nameStr)
	}

	return nil
}
