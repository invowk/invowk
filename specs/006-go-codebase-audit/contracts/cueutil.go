// SPDX-License-Identifier: MPL-2.0

//go:build ignore

// Package cueutil provides shared CUE parsing utilities.
//
// This is a CONTRACT FILE for planning purposes. It defines the API that will be
// implemented in pkg/cueutil/.
//
// The package consolidates the 3-step CUE parsing pattern used across invowkfile,
// invowkmod, and config packages.
package cueutil

import (
	"cuelang.org/go/cue"
)

// DefaultMaxFileSize is the default maximum file size for CUE parsing (5MB).
const DefaultMaxFileSize int64 = 5 * 1024 * 1024

// Option configures parsing behavior.
type Option func(*parseOptions)

// WithMaxFileSize sets the maximum allowed file size.
// Default is DefaultMaxFileSize (5MB).
func WithMaxFileSize(size int64) Option

// WithConcrete sets whether all values must be concrete after unification.
// Default is true (require concrete values).
//
// Set to false for config files where some fields may be optional and
// unset values are acceptable.
func WithConcrete(concrete bool) Option

// WithFilename sets the filename for error messages.
// This appears in CUE error output to help users locate issues.
func WithFilename(name string) Option

// ParseResult contains the result of a successful CUE parse operation.
type ParseResult[T any] struct {
	// Value is the decoded Go struct.
	Value *T

	// Unified is the unified CUE value, available for advanced use cases
	// such as extracting additional metadata or performing custom validation.
	Unified cue.Value
}

// ParseAndDecode performs the 3-step CUE parsing flow:
//
//  1. Compile the embedded schema
//  2. Compile user data and unify with schema
//  3. Validate and decode to Go struct
//
// Parameters:
//   - schema: The embedded CUE schema bytes (from //go:embed)
//   - data: The user-provided CUE file bytes
//   - schemaPath: The path to the root definition (e.g., "#Invowkfile", "#Config")
//   - opts: Optional configuration
//
// Returns:
//   - *ParseResult[T] containing the decoded struct and unified CUE value
//   - error with formatted path information if parsing fails
//
// Example usage:
//
//	//go:embed invowkfile_schema.cue
//	var schemaBytes []byte
//
//	result, err := cueutil.ParseAndDecode[Invowkfile](
//	    schemaBytes,
//	    userFileBytes,
//	    "#Invowkfile",
//	    cueutil.WithFilename("invowkfile.cue"),
//	)
//	if err != nil {
//	    return nil, err  // Error includes CUE path for debugging
//	}
//	return result.Value, nil
func ParseAndDecode[T any](schema, data []byte, schemaPath string, opts ...Option) (*ParseResult[T], error)

// CheckFileSize verifies that data does not exceed the specified maximum size.
// Returns an error if the size limit is exceeded.
//
// This is exposed for use cases where the caller needs to check size before
// reading the full file (e.g., when streaming).
func CheckFileSize(data []byte, maxSize int64, filename string) error

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
func FormatError(err error, filePath string) error

// --- Helper Types for Error Context ---

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
func (e *ValidationError) Error() string

// Unwrap returns nil (ValidationError is a leaf error).
func (e *ValidationError) Unwrap() error

// --- Internal Helpers (documented for implementation reference) ---

// formatPath converts a CUE path slice to JSON path notation.
// Example: ["cmds", "0", "script"] → "cmds[0].script"
//
// This is an internal function used by FormatError.
// func formatPath(path []string) string

// compileSchema compiles the embedded schema and extracts the root definition.
// Returns an error if schema compilation fails.
//
// This is an internal function used by ParseAndDecode.
// func compileSchema(ctx *cue.Context, schema []byte, schemaPath string) (cue.Value, error)

// unifyAndValidate unifies user data with schema and validates the result.
// Returns the unified value or a formatted error.
//
// This is an internal function used by ParseAndDecode.
// func unifyAndValidate(schema, userValue cue.Value, concrete bool) (cue.Value, error)
