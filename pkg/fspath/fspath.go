// SPDX-License-Identifier: MPL-2.0

// Package fspath provides typed wrappers around path/filepath functions that
// accept and return types.FilesystemPath. Each wrapper centralizes the single
// //goplint:ignore annotation so callers get typed-in/typed-out path operations
// without needing per-site suppression directives.
package fspath

import (
	"fmt"
	"path/filepath"

	"github.com/invowk/invowk/pkg/types"
)

// Join wraps filepath.Join, accepting and returning types.FilesystemPath.
// The returned path inherits validity from its typed input components.
func Join(elem ...types.FilesystemPath) types.FilesystemPath {
	strs := make([]string, len(elem))
	for i, e := range elem {
		strs[i] = string(e)
	}
	return types.FilesystemPath(filepath.Join(strs...)) //goplint:ignore -- derived from typed inputs
}

// JoinStr wraps filepath.Join, accepting a typed base path and raw string
// segments. Use this when joining a validated path with literal constants
// (e.g., "invowkmod.cue") or OS-provided file names (e.g., from os.ReadDir).
func JoinStr(base types.FilesystemPath, elem ...string) types.FilesystemPath {
	parts := make([]string, 1, 1+len(elem))
	parts[0] = string(base)
	parts = append(parts, elem...)
	return types.FilesystemPath(filepath.Join(parts...)) //goplint:ignore -- derived from typed base + string segments
}

// Dir wraps filepath.Dir for FilesystemPath.
func Dir(p types.FilesystemPath) types.FilesystemPath {
	return types.FilesystemPath(filepath.Dir(string(p))) //goplint:ignore -- derived from typed input
}

// Abs wraps filepath.Abs for FilesystemPath. Returns an error if the
// underlying OS call fails.
func Abs(p types.FilesystemPath) (types.FilesystemPath, error) {
	abs, err := filepath.Abs(string(p))
	if err != nil {
		return "", fmt.Errorf("resolving absolute path: %w", err)
	}
	return types.FilesystemPath(abs), nil //goplint:ignore -- OS-resolved absolute path
}

// Clean wraps filepath.Clean for FilesystemPath.
func Clean(p types.FilesystemPath) types.FilesystemPath {
	return types.FilesystemPath(filepath.Clean(string(p))) //goplint:ignore -- derived from typed input
}

// FromSlash wraps filepath.FromSlash for FilesystemPath. Converts forward
// slashes to the OS-specific path separator.
func FromSlash(p types.FilesystemPath) types.FilesystemPath {
	return types.FilesystemPath(filepath.FromSlash(string(p))) //goplint:ignore -- derived from typed input
}

// IsAbs wraps filepath.IsAbs for FilesystemPath.
func IsAbs(p types.FilesystemPath) bool {
	return filepath.IsAbs(string(p))
}
