// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// containModuleScriptRead runs the physical containment check of a module
// script read (SC-01) and turns its result into the resolver's error: nil when
// the injected reader may run, the traversal error when the path escapes or is
// an unresolvable link, and a script read error for any other OS error.
func containModuleScriptRead(selected ScriptFilePath, scriptPath, modulePath FilesystemPath) error {
	if _, err := checkModuleFilePhysicalContainment(scriptPath, modulePath); err != nil {
		if errors.Is(err, ErrScriptPathTraversal) {
			return err
		}
		return scriptFileReadError(selected, scriptPath, err)
	}
	return nil
}

// checkModuleFilePhysicalContainment judges a module file path physically: it
// opens an os.Root on the module directory and runs Stat on the path relative
// to it, so every link on the path is followed by the filesystem itself and an
// escape is reported by os.Root, never decided by comparing path strings (which
// macOS /var -> /private/var aliases and case-insensitive names defeat). The
// path must already have passed validateScriptPathContainment.
//
// The result is one of four (design D2 of module-read-physical-containment):
//   - contained: absent=false, err=nil; the path resolves inside the module;
//   - absent: absent=true, err=nil; the module directory is missing, or
//     nothing exists at the path (Stat and Lstat both report not-exist). The
//     caller's reader then reports the read error, and a virtual or fake
//     filesystem whose module directory is not on disk keeps working;
//   - escaping: an error wrapping ErrScriptPathTraversal; os.Root rejected the
//     path (a non-OS error, such as "path escapes from parent", which also
//     covers a link with an absolute target even inside the module), or the
//     path is a link that does not resolve (Stat fails while Lstat succeeds:
//     dangling, a broken chain or a cycle);
//   - denied: any other OS error, where the path itself cannot be inspected
//     (Lstat fails too, for example EACCES on a directory). It is returned
//     unwrapped for the caller to report as a read error.
func checkModuleFilePhysicalContainment(scriptPath, modulePath FilesystemPath) (absent bool, err error) {
	rel, err := filepath.Rel(string(modulePath), string(scriptPath))
	if err != nil {
		return false, fmt.Errorf("%w: failed to resolve path relative to module: %w", ErrScriptPathTraversal, err)
	}
	root, err := os.OpenRoot(string(modulePath))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return true, nil
		}
		return false, fmt.Errorf("open module root: %w", err)
	}
	defer func() { _ = root.Close() }() //nolint:errcheck // read-only os.Root; a close error is not actionable

	_, statErr := root.Stat(rel)
	if statErr == nil {
		return false, nil
	}
	if !isOSError(statErr) {
		return false, moduleFileEscapeError(scriptPath, modulePath, "resolves outside module", statErr)
	}
	_, lstatErr := root.Lstat(rel)
	switch {
	case lstatErr == nil && errors.Is(statErr, fs.ErrNotExist):
		return false, moduleFileEscapeError(scriptPath, modulePath, "is a dangling link in module", statErr)
	case lstatErr == nil:
		return false, moduleFileEscapeError(scriptPath, modulePath, "is unresolvable (link does not resolve) in module", statErr)
	case errors.Is(statErr, fs.ErrNotExist) && errors.Is(lstatErr, fs.ErrNotExist):
		return true, nil
	default:
		return false, fmt.Errorf("check module file: %w", statErr)
	}
}

// moduleFileEscapeError wraps ErrScriptPathTraversal with the path, the
// module, the case, and the os.Root error that decided it.
//
//goplint:ignore -- reason is one of three fixed display phrases for the error message.
func moduleFileEscapeError(scriptPath, modulePath FilesystemPath, reason string, cause error) error {
	return fmt.Errorf("%w: '%s' %s '%s': %w", ErrScriptPathTraversal, scriptPath, reason, modulePath, cause)
}

// isOSError reports whether err carries an operating-system error number.
// os.Root reports a path that leaves the root with a plain (non-errno) error,
// on every platform; a missing name, a permission error or a link cycle is an
// errno.
func isOSError(err error) bool {
	_, ok := errors.AsType[syscall.Errno](err) //nolint:errcheck // only the type-match verdict matters
	return ok
}
