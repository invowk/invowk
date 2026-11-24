// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// errRootResolverPanic marks a panic recovered from os.Root path resolution.
//
// Go 1.27's os.Root on Windows panics (index out of range in doInRoot) when the
// last path component is a link whose target resolves to the directory that
// holds it, such as a/x -> "." or a/b/x -> "../b": rootCleanPath folds the
// target into the already-traversed prefix, and the resolution loop indexes
// past the shortened path. Such a link resolves to a directory inside the
// root, which no script or env read can return content from.
var errRootResolverPanic = errors.New("os.Root path resolution panicked")

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

	_, statErr := rootStat(root.Stat, rel)
	if statErr == nil {
		return false, nil
	}
	if errors.Is(statErr, errRootResolverPanic) {
		return judgeRootResolverPanic(scriptPath, modulePath, statErr)
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

// rootStat runs stat (os.Root.Stat) on rel and turns a panic into an error
// wrapping errRootResolverPanic, so a module link cannot crash invowk.
//
//goplint:ignore -- rel is a transient os.Root-relative name, as with os.Root.Stat.
func rootStat(stat func(string) (fs.FileInfo, error), rel string) (fi fs.FileInfo, err error) {
	defer func() {
		if r := recover(); r != nil {
			fi, err = nil, fmt.Errorf("%w: %v", errRootResolverPanic, r)
		}
	}()
	return stat(rel)
}

// judgeRootResolverPanic decides a path whose os.Root Stat panicked. The panic
// only arises for a link that resolves to a directory inside the root, and a
// directory yields no content, so the path is passed on (contained) when
// os.Stat reports a directory and filepath.EvalSymlinks places it under the
// resolved module directory; the reader then reports the read error, as it does
// on the other platforms. Anything else fails closed as an escape: a path that
// is not a directory, a resolution error, a directory EvalSymlinks resolves
// outside the module. A link EvalSymlinks may leave unresolved (a Windows
// junction) is passed on by the os.Stat check alone, which is safe only
// because a directory holds no content.
func judgeRootResolverPanic(scriptPath, modulePath FilesystemPath, cause error) (absent bool, err error) {
	if fi, statErr := os.Stat(string(scriptPath)); statErr == nil && fi.IsDir() && resolvesUnder(scriptPath, modulePath) {
		return false, nil
	}
	return false, moduleFileEscapeError(scriptPath, modulePath, "is unresolvable (os.Root failed) in module", cause)
}

// resolvesUnder reports whether path, with every link resolved, is dir or lies
// below it (dir resolved the same way). Any resolution error is false.
func resolvesUnder(path, dir FilesystemPath) bool {
	resolvedPath, err := filepath.EvalSymlinks(string(path))
	if err != nil {
		return false
	}
	resolvedDir, err := filepath.EvalSymlinks(string(dir))
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(resolvedDir, resolvedPath)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
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
// errno. On Windows, os.Root also rejects a reserved device name (AUX, NUL,
// CON, COM1 and the like) with that plain error, so such a module file is
// reported as an escape: fail-closed, with a misleading but safe message.
func isOSError(err error) bool {
	_, ok := errors.AsType[syscall.Errno](err) //nolint:errcheck // only the type-match verdict matters
	return ok
}
