// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf8"

	"github.com/invowk/invowk/pkg/invowkfile"
)

// virtualPathNotDirectoryError refuses a no-follow request that names a
// directory by its form (a trailing separator, ".", "..") when its resolved
// path exists and is not a directory. Its text is "<path>: not a directory"
// on every platform, and it unwraps to syscall.ENOTDIR, whose own text is the
// system's (on Windows "The system cannot find the path specified.").
type virtualPathNotDirectoryError struct {
	//goplint:ignore -- raw OS path of the refused request, used only in the error text.
	path string
}

// Error returns "<path>: not a directory".
func (e *virtualPathNotDirectoryError) Error() string {
	return e.path + ": not a directory"
}

// Unwrap returns syscall.ENOTDIR.
func (e *virtualPathNotDirectoryError) Unwrap() error {
	return syscall.ENOTDIR
}

// resolveNoFollow resolves path for an operation that does not follow its
// last component (rm operands, mv sources, ln's link name, the -L/-h tests;
// design D1 of virtual-path-no-follow-operands):
//
//  1. A request that ends in a path separator, or whose last element is "."
//     or "..", is resolved in full (resolve), and dirCheck is true: the
//     caller then refuses it when the result exists and is not a directory.
//     This is read on the raw request, before expansion and cleaning, which
//     both drop a trailing separator.
//  2. Otherwise anchors and declared names are expanded and the path is made
//     absolute and cleaned.
//  3. A path that is its own parent (a volume root) is resolved in full.
//  4. The parent is resolved with normalizeVirtualPathForAccess: the strict
//     physical walk under restricted access, with the full-access fallback
//     otherwise. Its error is the request's error.
//  5. The result is the resolved parent joined with the final name, which
//     is never read, so it may be a link, a dangling link, a link in a cycle
//     or a reparse point whose target cannot be read.
//
//goplint:ignore -- filesystem boundary helper: raw OS path strings in and out.
func (r virtualPathResolver) resolveNoFollow(path, cwd string) (resolved string, dirCheck bool, err error) {
	if virtualPathNamesDirectory(path) {
		resolved, err = r.resolve(path, cwd)
		return resolved, true, err
	}
	expanded, err := r.expand(path)
	if err != nil {
		return "", false, err
	}
	cleaned, err := absoluteVirtualPath(expanded, cwd)
	if err != nil {
		return "", false, err
	}
	parent := filepath.Dir(cleaned)
	if parent == cleaned {
		resolved, err = normalizeVirtualPathForAccess(cleaned, "", r.access)
		return resolved, false, err
	}
	resolvedParent, err := normalizeVirtualPathForAccess(parent, "", r.access)
	if err != nil {
		return "", false, err
	}
	return filepath.Join(resolvedParent, filepath.Base(cleaned)), false, nil
}

// validateNoFollow is validate for an operation that does not follow its
// last component: it resolves path with resolveNoFollow and applies the same
// allowed-root check and denial error as validate, on that path. For a
// request that ends in a separator, ".", or ".." (resolveNoFollow step 1) it
// then stats the resolved path, which holds no trailing separator, and
// refuses with an error that wraps syscall.ENOTDIR and names that path when
// it exists and is not a directory: os.RemoveAll strips a trailing separator,
// so `rm -r link/` would otherwise remove a link's target file. Otherwise the
// separator the request ended in, if any, is put back on the returned path.
//
//goplint:ignore -- filesystem boundary helper: raw OS path strings in and out.
func (v virtualPathValidator) validateNoFollow(cwd, path string) (string, error) {
	resolved, dirCheck, err := v.resolver.resolveNoFollow(path, cwd)
	if err != nil {
		return "", err
	}
	if !v.withinAllowedRoots(resolved) {
		return "", virtualPolicyDenyError(errVirtualPathDenied, resolved)
	}
	if !dirCheck {
		return resolved, nil
	}
	if info, statErr := os.Stat(resolved); statErr == nil && !info.IsDir() {
		return "", &virtualPathNotDirectoryError{path: resolved}
	}
	if endsInVirtualPathSeparator(path) && !endsInVirtualPathSeparator(resolved) {
		resolved += string(filepath.Separator)
	}
	return resolved, nil
}

// withinAllowedRoots reports whether path may be used: always under full
// access, and under restricted access when it lies inside an allowed root.
//
//goplint:ignore -- raw OS path string.
func (v virtualPathValidator) withinAllowedRoots(path string) bool {
	if v.resolver.access == invowkfile.VirtualFilesystemAccessFull {
		return true
	}
	for _, root := range v.resolver.allowedRoots {
		if pathWithin(root, path) {
			return true
		}
	}
	return false
}

// virtualPathNamesDirectory reports whether a raw request names a directory
// by its form: it ends in a path separator, or its last element is "." or
// "..".
//
//goplint:ignore -- raw OS path string.
func virtualPathNamesDirectory(path string) bool {
	if endsInVirtualPathSeparator(path) {
		return true
	}
	last := path[strings.LastIndexFunc(path, func(r rune) bool { return r < utf8.RuneSelf && os.IsPathSeparator(uint8(r)) })+1:]
	return last == "." || last == virtualPathParent
}

//goplint:ignore -- raw OS path string.
func endsInVirtualPathSeparator(path string) bool {
	return path != "" && os.IsPathSeparator(path[len(path)-1])
}
