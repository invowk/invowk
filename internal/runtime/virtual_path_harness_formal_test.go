// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// TestVirtualPathHarness_F14DeepSymlink records finding F14: normalizeExistingOrParent
// evaluates only the path and its parent, so a path with two or more missing
// components below an escaping symlink is judged lexically and accepted, then the
// write lands outside the allowed root. This asserts today's behaviour.
func TestVirtualPathHarness_F14DeepSymlink(t *testing.T) {
	t.Parallel()
	base := fstree.TempRoot(t)
	root := mustMkdir(t, filepath.Join(base, "root"))
	outside := mustMkdir(t, filepath.Join(base, "outside"))
	if err := os.Symlink(outside, filepath.Join(root, "x")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	validator := virtualPathValidator{resolver: virtualPathResolver{
		allowedRoots: normalizedRoots([]string{root}),
		access:       invowkfile.VirtualFilesystemAccessRestricted,
	}}

	// depth 1 (parent evaluated) is correctly denied.
	if _, err := validator.validate(root, filepath.Join(root, "x", "a")); err == nil {
		t.Fatal("expected the one-level path through the symlink to be denied")
	}
	// depth 2+ (path and parent both missing) falls back to lexical and is accepted.
	normalized, err := validator.validate(root, filepath.Join(root, "x", "a", "b"))
	if err != nil {
		t.Fatalf("F14 did not reproduce: deep path denied: %v", err)
	}
	// Writing lands physically outside the allowed root, through the symlink.
	if mkErr := os.MkdirAll(normalized, 0o755); mkErr != nil {
		t.Fatalf("mkdir normalized: %v", mkErr)
	}
	if _, statErr := os.Stat(filepath.Join(outside, "a", "b")); statErr != nil {
		t.Fatalf("F14 did not reproduce: no directory created outside the root: %v", statErr)
	}
}

// TestVirtualPathHarness_F15WorkdirWidening records finding F15: under restricted
// access a module-declared workdir outside the module is added to the allowed
// roots, so an access below that workdir is accepted even though a workdir is a
// cwd, not a read root.
func TestVirtualPathHarness_F15WorkdirWidening(t *testing.T) {
	t.Parallel()
	base := fstree.TempRoot(t)
	scriptBase := mustMkdir(t, filepath.Join(base, "module"))

	// With the filesystem root as workdir ("/", or the volume root on Windows),
	// the resolver's allowed roots include it, so a read of a path anywhere on
	// that volume is admitted under restricted access.
	fsRoot := filepath.VolumeName(base) + string(filepath.Separator)
	resolver, err := newVirtualPathResolverForFilesystem(fsRoot, scriptBase, invowkfile.VirtualFilesystemConfig{
		Access: invowkfile.VirtualFilesystemAccessRestricted,
	})
	if err != nil {
		t.Fatalf("build resolver: %v", err)
	}
	if !slices.Contains(resolver.allowedRoots, fsRoot) {
		t.Fatalf("F15 precondition: workdir %q not added to allowed roots: %v", fsRoot, resolver.allowedRoots)
	}
	validator := virtualPathValidator{resolver: resolver}
	if _, err := validator.validate(scriptBase, filepath.Join(fsRoot, "etc", "hostname")); err != nil {
		t.Fatalf("F15 did not reproduce: an out-of-module path was denied under restricted access: %v", err)
	}
}
