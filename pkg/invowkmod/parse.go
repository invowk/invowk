// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/invowk/invowk/pkg/cueutil"
	"github.com/invowk/invowk/pkg/fspath"
	"github.com/invowk/invowk/pkg/types"
)

// ParseInvowkmod reads and parses module metadata from invowkmod.cue at the given path.
func ParseInvowkmod(path types.FilesystemPath) (*Invowkmod, error) {
	data, err := os.ReadFile(string(path))
	if err != nil {
		return nil, fmt.Errorf("failed to read invowkmod at %s: %w", path, err)
	}

	return ParseInvowkmodBytes(data, path)
}

// ParseInvowkmodBytes parses module metadata content from bytes.
// Uses cueutil.ParseAndDecodeString for the 3-step CUE parsing flow:
// compile schema -> compile user data -> validate and decode.
func ParseInvowkmodBytes(data []byte, path types.FilesystemPath) (*Invowkmod, error) {
	result, err := cueutil.ParseAndDecodeString[Invowkmod](
		invowkmodSchema,
		data,
		"#Invowkmod",
		cueutil.WithFilename(string(path)),
	)
	if err != nil {
		return nil, err
	}

	meta := result.Value
	meta.FilePath = path

	// Validate module requirement paths for security
	// [GO-ONLY] Path traversal prevention and cross-platform path handling require Go.
	// CUE cannot perform filesystem operations or cross-platform path normalization.
	for i, req := range meta.Requires {
		if req.Path != "" {
			if err := req.Path.Validate(); err != nil {
				return nil, fmt.Errorf("requires[%d].path: %w in invowkmod at %s", i, err, path)
			}
		}
	}

	return meta, nil
}

// ParseModuleMetadataOnly reads and parses only the module metadata (invowkmod.cue) from a module directory.
// This is useful when you only need module identity and dependencies, not commands.
// Returns ErrInvowkmodNotFound if invowkmod.cue doesn't exist.
func ParseModuleMetadataOnly(modulePath types.FilesystemPath) (*Invowkmod, error) {
	invowkmodPath := fspath.JoinStr(modulePath, "invowkmod.cue")
	if _, err := os.Stat(string(invowkmodPath)); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrInvowkmodNotFound
		}
		return nil, fmt.Errorf("failed to check invowkmod at %s: %w", invowkmodPath, err)
	}
	return ParseInvowkmod(invowkmodPath)
}

// HasInvowkfile checks if a module directory contains an invowkfile.cue.
func HasInvowkfile(modulePath types.FilesystemPath) bool {
	invowkfilePath := filepath.Join(string(modulePath), "invowkfile.cue")
	_, err := os.Stat(invowkfilePath)
	return err == nil
}

// InvowkfilePath returns the path to invowkfile.cue in a module directory.
func InvowkfilePath(modulePath types.FilesystemPath) types.FilesystemPath {
	return fspath.JoinStr(modulePath, "invowkfile.cue")
}

// InvowkmodPath returns the path to invowkmod.cue in a module directory.
//
//nolint:revive // Name is intentional for consistency with Module.InvowkmodPath field/method
func InvowkmodPath(modulePath types.FilesystemPath) types.FilesystemPath {
	return fspath.JoinStr(modulePath, "invowkmod.cue")
}
