// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	slashpath "path"
	"strings"

	"github.com/invowk/invowk/pkg/platform"
)

// ValidateFilename checks if a filename is valid across platforms.
func ValidateFilename(name string) error {
	if name == "" {
		return errors.New("filename cannot be empty")
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
			return errors.New("filename contains control character")
		}
	}

	// Check for Windows reserved names
	if platform.IsWindowsReservedName(name) {
		return fmt.Errorf("filename '%s' is reserved on Windows", name)
	}

	// Check for names ending with space or period (invalid on Windows)
	if strings.HasSuffix(name, " ") || strings.HasSuffix(name, ".") {
		return errors.New("filename cannot end with space or period")
	}

	return nil
}

// ValidateContainerfilePath validates a containerfile path for security.
// [GO-ONLY] Path traversal prevention MUST be in Go because it requires:
// 1. Cross-platform path separator handling
// 2. Validation that must remain deterministic across host operating systems
// CUE can only validate static patterns (like !strings.Contains("..")) but cannot
// reliably canonicalize slash and backslash path dialects.
//
// It ensures paths are relative, don't escape the invowkfile directory,
// and contain valid filename characters.
func ValidateContainerfilePath(containerfile, baseDir string) error {
	_ = baseDir // Kept for API compatibility with validation call sites.
	if containerfile == "" {
		return nil
	}

	// Check length limit
	if len(containerfile) > MaxPathLength {
		return fmt.Errorf("containerfile path too long (%d chars, max %d)", len(containerfile), MaxPathLength)
	}

	// Containerfile path must be relative (use cross-platform check)
	if isAbsolutePath(containerfile) {
		return errors.New("containerfile path must be relative, not absolute")
	}

	// Check for null bytes (security)
	if strings.ContainsRune(containerfile, '\x00') {
		return errors.New("containerfile path contains null byte")
	}

	// Normalize separators before cleaning so a raw parent-directory segment
	// cannot be hidden by slashpath.Clean.
	normalizedPath := strings.ReplaceAll(containerfile, "\\", "/")
	if containsParentPathSegment(normalizedPath) {
		return fmt.Errorf("containerfile path %q contains parent-directory segment '..'", containerfile)
	}
	cleanPath := slashpath.Clean(normalizedPath)

	// Validate the filename component
	return ValidateFilename(slashpath.Base(cleanPath))
}

//goplint:ignore -- low-level path scanner intentionally works on normalized raw path text before typed construction.
func containsParentPathSegment(path string) bool {
	for segment := range strings.SplitSeq(path, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
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
		return errors.New("env file path cannot be empty")
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
		return errors.New("env file path contains null byte")
	}

	// Check for path traversal sequences using slash semantics so backslash
	// traversal is rejected identically on every host OS.
	normalizedPath := strings.ReplaceAll(cleanPath, "\\", "/")
	if containsParentPathSegment(normalizedPath) {
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
// It delegates to BinaryName so tool dependency invariants stay owned by the
// value object used by programmatic callers and parsed CUE alike.
func ValidateToolName(name BinaryName) error {
	return name.Validate()
}

// ValidateCommandDependencyName validates a bare command dependency name.
func ValidateCommandDependencyName(name CommandName) error {
	s := string(name)
	if s == "" {
		return errors.New("command name cannot be empty")
	}
	if len(s) > MaxNameLength {
		return fmt.Errorf("command name too long (%d chars, max %d)", len(s), MaxNameLength)
	}
	if !cmdDependencyNameRegex.MatchString(s) {
		return fmt.Errorf("command name '%s' is invalid (must start with letter, can include alphanumeric, underscores, hyphens, spaces)", s)
	}
	return nil
}

// ValidateCommandDependencyRef validates a command dependency reference.
// [CUE-VALIDATED] CUE owns the static reference grammar. Invowk parses the
// value after decode so dependency resolution can distinguish bare local refs
// from explicit @source command refs.
func ValidateCommandDependencyRef(ref CommandDependencyRef) error {
	return ref.Validate()
}

// isAbsolutePath checks if a path is absolute in any of the four dialects
// the codebase must accept as input — Unix-rooted ("/foo"), Windows-drive
// ("C:\foo" or "C:/foo"), Windows-rooted ("\foo"), and UNC ("\\server\share").
// Unlike filepath.IsAbs(), this function works cross-platform: every dialect
// is detected regardless of the host operating system. This is essential
// for security validation of user-provided paths that may originate from
// different platforms — without it, validators silently accept Windows-style
// absolutes on Linux/macOS and treat them as relative segments, the v0.10.0
// cross-platform divergence class.
func isAbsolutePath(path string) bool {
	if path == "" {
		return false
	}

	// Unix-style absolute path. Also catches UNC ("\\server\share") and
	// Windows-rooted ("\foo") after backslash normalization below.
	if path[0] == '/' || path[0] == '\\' {
		return true
	}

	// Windows-style absolute path: drive letter + colon + path separator
	// Examples: "C:\Users" or "C:/Users".
	if len(path) >= 3 && isWindowsDriveLetter(path[0]) && path[1] == ':' {
		sep := path[2]
		return sep == '\\' || sep == '/'
	}

	return false
}

// isWindowsDriveLetter returns true if c is a valid Windows drive letter.
func isWindowsDriveLetter(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}
