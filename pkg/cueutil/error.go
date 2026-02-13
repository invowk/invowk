// SPDX-License-Identifier: MPL-2.0

package cueutil

import (
	"fmt"
	"strings"

	"cuelang.org/go/cue/errors"
)

// ValidationError represents a CUE validation error with context.
type ValidationError struct {
	// FilePath is the file being validated.
	FilePath string

	// CUEPath is the JSON path to the invalid value (e.g., "cmds[0].name").
	CUEPath string

	// Message is the validation error message.
	Message string

	// Suggestion is an optional hint for fixing the error.
	Suggestion string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if e.CUEPath != "" {
		return fmt.Sprintf("%s: %s: %s", e.FilePath, e.CUEPath, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.FilePath, e.Message)
}

// Unwrap returns nil (ValidationError is a leaf error).
func (e *ValidationError) Unwrap() error {
	return nil
}

// FormatError formats a CUE error with JSON path prefixes for clear error messages.
//
// Error format: <file-path>: <json-path>: <message>
//
// Examples:
//   - invowkfile.cue: cmds[0].implementations[2].script: value exceeds maximum length
//   - config.cue: container.auto_provision.enabled: expected bool, got string
//
// This function is exposed for packages that need custom error formatting
// beyond what ParseAndDecode provides.
func FormatError(err error, filePath string) error {
	if err == nil {
		return nil
	}

	// Extract all CUE errors
	cueErrors := errors.Errors(err)
	if len(cueErrors) == 0 {
		// Fallback: not a CUE error, return as-is
		return fmt.Errorf("%s: %w", filePath, err)
	}

	var lines []string
	for _, e := range cueErrors {
		// Get the path to the problematic field
		path := errors.Path(e)
		pathStr := formatPath(path)
		msg := e.Error()

		// Remove redundant path prefix from message if present
		// CUE sometimes includes the path in the message itself
		if pathStr != "" && strings.HasPrefix(msg, pathStr) {
			msg = strings.TrimPrefix(msg, pathStr)
			msg = strings.TrimPrefix(msg, ":")
			msg = strings.TrimSpace(msg)
		}

		if pathStr != "" {
			lines = append(lines, fmt.Sprintf("%s: %s", pathStr, msg))
		} else {
			lines = append(lines, msg)
		}
	}

	if len(lines) == 1 {
		return fmt.Errorf("%s: %s", filePath, lines[0])
	}
	return fmt.Errorf("%s: validation failed:\n  %s", filePath, strings.Join(lines, "\n  "))
}

// formatPath converts a CUE error path to JSON-path notation for user-facing messages.
// CUE provides error paths as flat string slices (e.g., ["cmds", "0", "script"]) where
// numeric elements represent array indices. This function converts to JSON-path notation
// (e.g., "cmds[0].script") which is more familiar to users.
func formatPath(path []string) string {
	if len(path) == 0 {
		return ""
	}

	// Join with dots but handle array indices specially
	// The path from CUE is already in a good format like ["cmds", "0", "script"]
	// We want to produce "cmds[0].script"
	var result strings.Builder
	for i, part := range path {
		// Check if this looks like an array index (purely numeric)
		isIndex := true
		for _, c := range part {
			if c < '0' || c > '9' {
				isIndex = false
				break
			}
		}

		if isIndex && i > 0 {
			result.WriteString("[")
			result.WriteString(part)
			result.WriteString("]")
		} else {
			if i > 0 {
				result.WriteString(".")
			}
			result.WriteString(part)
		}
	}

	return result.String()
}

// CheckFileSize verifies that data does not exceed the specified maximum size.
// Returns an error if the size limit is exceeded.
//
// This is exposed for use cases where the caller needs to check size before
// reading the full file (e.g., when streaming).
func CheckFileSize(data []byte, maxSize int64, filename string) error {
	if int64(len(data)) > maxSize {
		return fmt.Errorf("%s: file size %d bytes exceeds maximum %d bytes",
			filename, len(data), maxSize)
	}
	return nil
}
