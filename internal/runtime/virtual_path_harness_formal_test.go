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
	fstree.Require(t, fstree.CapSymlink)
	base := fstree.TempRoot(t)
	root := mustMkdir(t, filepath.Join(base, "root"))
	outside := mustMkdir(t, filepath.Join(base, "outside"))
	fstree.MustSymlink(t, outside, filepath.Join(root, "x"))
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

// TestVirtualPathHarness_RootConstruction replays the resolver construction
// (rows allowedRoots and access) against the model's verdict: the real
// newVirtualPathResolverForFilesystem must accept a probe exactly when
// H.access = Full or locIsAllowedRoot[l, includeWorkdir = True]
// (VirtualPathHarness.als), and EffectiveAccess must map an omitted access mode
// to restricted (openspec/specs/virtual-filesystem-access/spec.md, "Omitted
// virtual filesystem config is restricted"). includeWorkdir = True is the
// current code (F15), so a workdir-only probe is accepted under restricted.
//
// Probes are classified by the model's location kinds from how they are built,
// never from other virtual_policy.go helpers:
//   - InReadRoot: a file in the script base, the declared path when one is
//     declared, and each automatically allowed anchor the resolver reports
//     (@config, @data, @cache, @state, @tmp; openspec cross-platform-path-anchors);
//   - InWorkdir: a file under the workdir only;
//   - OutsideAll: a path outside every root (and the undeclared path).
//
// The workdir, declared path, and outside probe are missing paths directly
// under the volume root, so no temp or anchor root encloses them. The per-OS
// anchor layout is not modelled (row anchors is unbound), so no anchor value is
// compared with a table.
func TestVirtualPathHarness_RootConstruction(t *testing.T) {
	t.Parallel()
	base := fstree.TempRoot(t)
	scriptBase := mustMkdir(t, filepath.Join(base, "module"))
	scriptFile := filepath.Join(scriptBase, "script.sh")
	if err := os.WriteFile(scriptFile, []byte("echo"), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}
	volume := filepath.VolumeName(base) + string(filepath.Separator)
	tag := filepath.Base(base)
	workDir := filepath.Join(volume, "invowk-work-"+tag)
	declared := filepath.Join(volume, "invowk-declared-"+tag)
	outside := filepath.Join(volume, "invowk-outside-"+tag, "a", "b")

	const (
		inReadRoot = "InReadRoot"
		inWorkdir  = "InWorkdir"
		outsideAll = "OutsideAll"
	)
	accesses := map[string]invowkfile.VirtualFilesystemAccess{
		"omitted":    "",
		"restricted": invowkfile.VirtualFilesystemAccessRestricted,
		"full":       invowkfile.VirtualFilesystemAccessFull,
	}
	wantEffective := map[string]invowkfile.VirtualFilesystemAccess{
		"omitted":    invowkfile.VirtualFilesystemAccessRestricted,
		"restricted": invowkfile.VirtualFilesystemAccessRestricted,
		"full":       invowkfile.VirtualFilesystemAccessFull,
	}
	for name, access := range accesses {
		for _, withDeclared := range []bool{false, true} {
			config := invowkfile.VirtualFilesystemConfig{Access: access}
			if withDeclared {
				config.Paths = invowkfile.VirtualFilesystemPaths{"DECLARED": invowkfile.VirtualFilesystemPath(declared)}
			}
			if got := config.EffectiveAccess(); got != wantEffective[name] {
				t.Errorf("access %s: EffectiveAccess = %q, want %q", name, got, wantEffective[name])
			}
			resolver, err := newVirtualPathResolverForFilesystem(workDir, scriptBase, config)
			if err != nil {
				t.Fatalf("access %s, declared %v: build resolver: %v", name, withDeclared, err)
			}

			probes := map[string]string{
				scriptFile:                            inReadRoot,
				filepath.Join(workDir, "f"):           inWorkdir,
				outside:                               outsideAll,
				filepath.Join(declared, "x", "probe"): outsideAll,
			}
			if withDeclared {
				probes[filepath.Join(declared, "x", "probe")] = inReadRoot
			}
			for _, anchor := range []string{virtualAnchorConfig, virtualAnchorData, virtualAnchorCache, virtualAnchorState, virtualAnchorTmp} {
				root := resolver.anchors[anchor]
				if root == "" {
					t.Fatalf("access %s: the resolver reports no %s anchor", name, anchor)
				}
				probes[root] = inReadRoot
			}

			full := wantEffective[name] == invowkfile.VirtualFilesystemAccessFull
			validator := virtualPathValidator{resolver: resolver}
			for probe, loc := range probes {
				// H.access = Full or locIsAllowedRoot[l, includeWorkdir = True]
				want := full || loc == inReadRoot || loc == inWorkdir
				_, err := validator.validate(scriptBase, probe)
				if got := err == nil; got != want {
					t.Errorf("access %s, declared %v: probe %s (%s) accepted=%v, model %v (err %v)",
						name, withDeclared, probe, loc, got, want, err)
				}
			}
		}
	}
}
