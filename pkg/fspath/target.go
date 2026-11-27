// SPDX-License-Identifier: MPL-2.0

package fspath

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const nonRegularTargetErrMsg = "target is not a regular file"

// ErrNonRegularTarget indicates that an inspected path or its resolved symlink
// target is a directory, FIFO, socket, device, or other non-regular entry.
var ErrNonRegularTarget = errors.New(nonRegularTargetErrMsg)

// FileTarget describes a resolved writable regular-file destination (or an
// absent path) returned by InspectWritableTarget.
type FileTarget struct {
	// Path is the resolved regular-file path when Exists is true (or the original
	// path when Exists is false).
	Path string
	// Perm holds the target regular file's permission bits when Exists is true.
	Perm os.FileMode
	// Exists reports whether a regular file exists at Path (directly or via
	// symbolic link).
	Exists bool
	// WasSymlink reports whether the input path was a symbolic link.
	WasSymlink bool
}

// InspectWritableTarget checks path without following a dangling link silently
// and resolves any symbolic link to its canonical regular target file so
// AtomicWriteFile updates the resolved destination while keeping the symlink
// entry intact.
//
// Truly absent paths return FileTarget{Path: path, Exists: false}, nil. Dangling
// symlinks, circular symlinks, and symlinks (or direct paths) pointing to
// directories or special files fail closed with an error.
func InspectWritableTarget(path string) (FileTarget, error) {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return FileTarget{Path: path}, nil
		}
		return FileTarget{}, fmt.Errorf("inspect target %s: %w", path, err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		if !info.Mode().IsRegular() {
			return FileTarget{}, fmt.Errorf("%w: %s (%s)", ErrNonRegularTarget, path, info.Mode().Type())
		}
		return FileTarget{
			Path:   path,
			Perm:   info.Mode().Perm(),
			Exists: true,
		}, nil
	}

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return FileTarget{}, fmt.Errorf("resolve symlink %s: %w", path, err)
	}
	targetInfo, err := os.Lstat(resolved)
	if err != nil {
		return FileTarget{}, fmt.Errorf("inspect symlink target %s (from %s): %w", resolved, path, err)
	}
	if !targetInfo.Mode().IsRegular() {
		return FileTarget{}, fmt.Errorf("%w: %s via symlink %s (%s)", ErrNonRegularTarget, resolved, path, targetInfo.Mode().Type())
	}

	return FileTarget{
		Path:       resolved,
		Perm:       targetInfo.Mode().Perm(),
		Exists:     true,
		WasSymlink: true,
	}, nil
}
