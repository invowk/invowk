// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/invowk/invowk/pkg/fspath"
)

// ErrInvalidScriptFilePath is the sentinel error wrapped by InvalidScriptFilePathError.
var ErrInvalidScriptFilePath = errors.New("invalid script file path")

type (
	// ScriptFilePath is a module-relative script file reference.
	// It is resolved against the source module root and must not be absolute or
	// contain parent-directory segments.
	ScriptFilePath string

	// InvalidScriptFilePathError is returned when a script file path cannot be
	// safely resolved within a module.
	InvalidScriptFilePathError struct {
		Value  *ScriptFilePath
		Reason string
	}
)

// String returns the string representation of the script file path.
func (p ScriptFilePath) String() string { return string(p) }

// Validate returns nil when the script file path is a module-relative,
// non-empty path without parent-directory traversal.
//
//goplint:nonzero
func (p ScriptFilePath) Validate() error {
	path := strings.TrimSpace(string(p))
	if path == "" {
		return newInvalidScriptFilePathError(p, "must be non-empty")
	}
	if len(path) > MaxPathLength {
		return newInvalidScriptFilePathError(p, fmt.Sprintf("path too long (%d chars, max %d)", len(path), MaxPathLength))
	}
	if strings.ContainsRune(path, '\x00') {
		return newInvalidScriptFilePathError(p, "must not contain null bytes")
	}
	if isAbsolutePath(path) {
		return newInvalidScriptFilePathError(p, "must be relative to the module root")
	}
	normalized := strings.ReplaceAll(path, "\\", "/")
	if containsParentPathSegment(normalized) {
		return newInvalidScriptFilePathError(p, "must not contain parent-directory segments")
	}
	return nil
}

// ResolveFromModule resolves the script file reference against modulePath.
// Invalid absolute inputs are preserved as absolute paths so containment checks
// at file I/O boundaries can still fail closed if validation was bypassed.
func (p ScriptFilePath) ResolveFromModule(modulePath FilesystemPath) FilesystemPath {
	if modulePath == "" {
		return ""
	}
	path := strings.TrimSpace(string(p))
	if path == "" {
		return ""
	}
	if isAbsolutePath(path) {
		return FilesystemPath(path) //goplint:ignore -- resolution preserves raw invalid values for fail-closed containment checks.
	}
	normalized := strings.ReplaceAll(path, "\\", "/")
	return fspath.JoinStr(modulePath, filepath.FromSlash(normalized))
}

//goplint:ignore -- reason is human-readable validation detail for an error payload.
func newInvalidScriptFilePathError(value ScriptFilePath, reason string) *InvalidScriptFilePathError {
	return &InvalidScriptFilePathError{Value: &value, Reason: reason}
}

// Error implements the error interface for InvalidScriptFilePathError.
func (e *InvalidScriptFilePathError) Error() string {
	value := ScriptFilePath("")
	if e != nil && e.Value != nil {
		value = *e.Value
	}
	reason := ""
	if e != nil {
		reason = e.Reason
	}
	return fmt.Sprintf("invalid script file path %q: %s", value, reason)
}

// Unwrap returns script-file and filesystem-path sentinels for compatibility.
func (e *InvalidScriptFilePathError) Unwrap() error {
	return errors.Join(ErrInvalidScriptFilePath, ErrInvalidFilesystemPath)
}
