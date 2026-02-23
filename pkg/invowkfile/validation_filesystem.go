// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/pkg/platform"
)

// ValidateFilename checks if a filename is valid across platforms.
func ValidateFilename(name string) error {
	if name == "" {
		return fmt.Errorf("filename cannot be empty")
	}

	// Check length
	if len(name) > 255 {
		return fmt.Errorf("filename too long (%d chars, max 255)", len(name))
	}

	// Check for invalid characters (common across platforms)
	invalidChars := []byte{'<', '>', ':', '"', '|', '?', '*', '\x00'}
	for _, c := range invalidChars {
		if strings.ContainsRune(name, rune(c)) {
			return fmt.Errorf("filename contains invalid character '%c'", c)
		}
	}

	// Check for control characters
	for _, r := range name {
		if r < 32 {
			return fmt.Errorf("filename contains control character")
		}
	}

	// Check for Windows reserved names
	if platform.IsWindowsReservedName(name) {
		return fmt.Errorf("filename '%s' is reserved on Windows", name)
	}

	// Check for names ending with space or period (invalid on Windows)
	if strings.HasSuffix(name, " ") || strings.HasSuffix(name, ".") {
		return fmt.Errorf("filename cannot end with space or period")
	}

	return nil
}

// ValidateContainerfilePath validates a containerfile path for security.
// [GO-ONLY] Path traversal prevention MUST be in Go because it requires:
// 1. Access to filesystem operations (filepath.Join, filepath.Clean, filepath.Rel)
// 2. Knowledge of the baseDir (invowkfile directory) at runtime
// 3. Cross-platform path separator handling
// CUE can only validate static patterns (like !strings.Contains("..")) but cannot
// detect sophisticated path traversal via symlinks or normalized paths.
//
// It ensures paths are relative, don't escape the invowkfile directory,
// and contain valid filename characters.
func ValidateContainerfilePath(containerfile, baseDir string) error {
	if containerfile == "" {
		return nil
	}

	// Check length limit
	if len(containerfile) > MaxPathLength {
		return fmt.Errorf("containerfile path too long (%d chars, max %d)", len(containerfile), MaxPathLength)
	}

	// Containerfile path must be relative (use cross-platform check)
	if isAbsolutePath(containerfile) {
		return fmt.Errorf("containerfile path must be relative, not absolute")
	}

	// Check for null bytes (security)
	if strings.ContainsRune(containerfile, '\x00') {
		return fmt.Errorf("containerfile path contains null byte")
	}

	// Convert to native path separators and resolve
	nativePath := filepath.FromSlash(containerfile)
	fullPath := filepath.Join(baseDir, nativePath)
	cleanPath := filepath.Clean(fullPath)

	// Verify the resolved path stays within baseDir
	relPath, err := filepath.Rel(baseDir, cleanPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return fmt.Errorf("containerfile path '%s' escapes the invowkfile directory", containerfile)
	}

	// Validate the filename component
	return ValidateFilename(filepath.Base(containerfile))
}

// ValidateEnvFilePath validates an env file path for security.
// [GO-ONLY] Path traversal prevention and cross-platform path handling require Go.
// CUE cannot perform filesystem operations or cross-platform path normalization.
//
// Env file paths support an optional '?' suffix to mark the file as optional.
// It ensures paths are relative and don't contain path traversal sequences.
func ValidateEnvFilePath(filePath string) error {
	// Remove optional '?' suffix
	cleanPath := strings.TrimSuffix(filePath, "?")

	if cleanPath == "" {
		return fmt.Errorf("env file path cannot be empty")
	}

	// Check length limit
	if len(cleanPath) > MaxPathLength {
		return fmt.Errorf("env file path too long (%d chars, max %d)", len(cleanPath), MaxPathLength)
	}

	// Env file path must be relative (use cross-platform check)
	if isAbsolutePath(cleanPath) {
		return fmt.Errorf("env file path must be relative: %s", cleanPath)
	}

	// Check for null bytes (security)
	if strings.ContainsRune(cleanPath, '\x00') {
		return fmt.Errorf("env file path contains null byte")
	}

	// Check for path traversal sequences
	normalized := filepath.Clean(cleanPath)
	if strings.HasPrefix(normalized, "..") || strings.Contains(normalized, string(filepath.Separator)+"..") {
		return fmt.Errorf("env file path cannot contain '..': %s", filePath)
	}

	return nil
}

// ValidateFilepathDependency validates filepath dependency alternatives.
// [GO-ONLY] Security constraints (null bytes, length limits) require Go validation.
// [CUE-VALIDATED] Basic non-empty constraint is in CUE: alternatives: [...string & !=""]
// These paths are checked at runtime, but we validate basic security constraints.
func ValidateFilepathDependency(paths []FilesystemPath) error {
	for i, path := range paths {
		s := string(path)
		if s == "" {
			return fmt.Errorf("filepath alternative #%d cannot be empty", i+1)
		}

		if len(s) > MaxPathLength {
			return fmt.Errorf("filepath alternative #%d too long (%d chars, max %d)", i+1, len(s), MaxPathLength)
		}

		// Check for null bytes (security)
		if strings.ContainsRune(s, '\x00') {
			return fmt.Errorf("filepath alternative #%d contains null byte", i+1)
		}
	}
	return nil
}

// ValidateToolName validates a tool/binary name.
// [CUE-VALIDATED] Format validation (alphanumeric, can include . _ + -) is in CUE schema:
// alternatives: [...string & =~"^[a-zA-Z0-9][a-zA-Z0-9._+-]*$"]
// [GO-ONLY] Length limit validation (MaxNameLength) is Go-only because CUE schema
// doesn't enforce string length limits on tool names for simplicity.
func ValidateToolName(name string) error {
	// Length check is Go-only (not in CUE schema)
	if len(name) > MaxNameLength {
		return fmt.Errorf("tool name too long (%d chars, max %d)", len(name), MaxNameLength)
	}
	// Format validation is redundant with CUE but kept as defense-in-depth
	// for cases where this function is called outside the CUE parse flow.
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if !toolNameRegex.MatchString(name) {
		return fmt.Errorf("tool name '%s' is invalid (must be alphanumeric, can include . _ + -)", name)
	}
	return nil
}

// ValidateCommandDependencyName validates a command dependency name.
// [CUE-VALIDATED] Format validation is in CUE: alternatives: [...string & =~"^[a-zA-Z][a-zA-Z0-9_ -]*$"]
// [GO-ONLY] Length limit (MaxNameLength) is Go-only for defense-in-depth.
func ValidateCommandDependencyName(name string) error {
	if name == "" {
		return fmt.Errorf("command name cannot be empty")
	}
	// [GO-ONLY] Length limit - not in CUE schema
	if len(name) > MaxNameLength {
		return fmt.Errorf("command name too long (%d chars, max %d)", len(name), MaxNameLength)
	}
	if !cmdDependencyNameRegex.MatchString(name) {
		return fmt.Errorf("command name '%s' is invalid (must start with letter, can include alphanumeric, underscores, hyphens, spaces)", name)
	}
	return nil
}

// isAbsolutePath checks if a path is absolute in either Unix or Windows format.
// Unlike filepath.IsAbs(), this function works cross-platform: it detects both
// Unix-style paths (/etc/passwd) and Windows-style paths (C:\Windows or C:/Windows)
// regardless of the host operating system. This is essential for security validation
// of user-provided paths that may originate from different platforms.
func isAbsolutePath(path string) bool {
	if path == "" {
		return false
	}

	// Unix-style absolute path
	if path[0] == '/' {
		return true
	}

	// Windows-style absolute path: drive letter + colon + path separator
	// Examples: "C:\Users" or "C:/Users"
	if len(path) >= 3 && isWindowsDriveLetter(path[0]) && path[1] == ':' {
		sep := path[2]
		return sep == '\\' || sep == '/'
	}

	return false
}
