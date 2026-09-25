// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"pgregory.net/rapid"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// TestVirtualPathHarness_Property generates allowed roots, runtime-created
// symlinks, and deep non-existent paths at larger scope than the golden, and
// checks the real validator: under restricted access an accepted path resolves
// physically inside an allowed root, unless the acceptance is the recorded F14
// deep-missing-path-through-a-symlink hole.
func TestVirtualPathHarness_Property(t *testing.T) {
	t.Parallel()
	root := fstree.TempRoot(t)

	rapid.Check(t, func(rt *rapid.T) {
		dir, err := os.MkdirTemp(root, "case-*")
		if err != nil {
			rt.Fatalf("mkdtemp: %v", err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		allowed := filepath.Join(dir, "allowed")
		outside := filepath.Join(dir, "outside")
		for _, d := range []string{allowed, outside} {
			if err = os.MkdirAll(d, 0o755); err != nil {
				rt.Fatalf("mkdir: %v", err)
			}
		}
		validator := virtualPathValidator{resolver: virtualPathResolver{
			allowedRoots: normalizedRoots([]string{allowed}),
			access:       invowkfile.VirtualFilesystemAccessRestricted,
		}}

		// Optionally place an escaping symlink inside the allowed root.
		linkName := ""
		if rapid.Bool().Draw(rt, "escapingLink") {
			linkName = "x"
			if err = os.Symlink(outside, filepath.Join(allowed, linkName)); err != nil {
				rt.Skipf("symlink: %v", err)
			}
		}

		// Build a requested path: optionally through the link, with a random
		// number of trailing (missing) components.
		segs := rapid.IntRange(0, 3).Draw(rt, "missing")
		parts := []string{allowed}
		crossed := false
		if linkName != "" && rapid.Bool().Draw(rt, "useLink") {
			parts = append(parts, linkName)
			crossed = true
		}
		for range segs {
			parts = append(parts, "m")
		}
		reqPath := filepath.Join(parts...)

		normalized, err := validator.validate(allowed, reqPath)
		if err != nil {
			return // denied: safe
		}
		resolved, rerr := filepath.EvalSymlinks(normalized)
		if rerr != nil {
			// normalized is a non-existent path; check its deepest existing ancestor
			resolved = deepestExisting(normalized)
		}
		if fstree.Within(allowed, resolved) {
			return // contained
		}
		// The only accepted escape is F14: a deep (>=2 missing) path through the
		// escaping symlink, where normalizeExistingOrParent falls back to lexical.
		if !crossed || segs < 2 {
			rt.Fatalf("policy violation: %q accepted but resolves outside the root to %q (crossed=%v missing=%d)",
				reqPath, resolved, crossed, segs)
		}
	})
}

func deepestExisting(path string) string {
	for path != "" && path != string(filepath.Separator) {
		if _, err := os.Lstat(path); err == nil {
			resolved, rerr := filepath.EvalSymlinks(path)
			if rerr == nil {
				return resolved
			}
			return path
		}
		path = filepath.Dir(path)
	}
	return path
}
