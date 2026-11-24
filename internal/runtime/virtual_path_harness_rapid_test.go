// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// Shapes of the link TestVirtualPathHarness_Property places at allowed/x. T is
// the link's target directory, inside or outside the allowed root.
const (
	propNoLink              = "no link"
	propExisting            = "T"
	propParentOfExisting    = "T/../T"
	propDangling            = "T/gone"
	propParentBeforeMissing = "T/../T/gone"
	propParentAfterMissing  = "T/nope/../gone"
	propTwoLinks            = "nope/../toout/new"
	propJunction            = "junction to T"

	// Classes of the oracle.
	propInside       = "resolves inside the root"
	propOutside      = "resolves outside the root"
	propUnresolvable = "cannot be resolved"
)

// TestVirtualPathHarness_Property generates allowed roots, links and deep
// non-existent paths at larger scope than the golden, and compares the real
// validator with an oracle in both directions. The oracle is the physical
// location of the request's deepest existing ancestor, computed from how the
// test built the tree, never by calling the resolver:
//   - resolves inside the root: a denial fails the property, and so does a
//     validated path other than the one the tree implies;
//   - resolves outside the root: an acceptance fails it, at every depth (there
//     is no exemption for a deep path through a link);
//   - cannot be resolved (a link target with ".." after a missing component):
//     an acceptance fails it.
//
// The links escape or stay inside, have relative and absolute targets, dangle,
// and hold ".." before and after a missing component; on Windows they are also
// junctions. An accepted path is created, as a script would, and after every
// case nothing may exist outside the root that the test did not put there.
func TestVirtualPathHarness_Property(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	shapes := []string{
		propNoLink, propExisting, propParentOfExisting, propDangling,
		propParentBeforeMissing, propParentAfterMissing, propTwoLinks,
	}
	if harnessJunctions(t) {
		shapes = append(shapes, propJunction)
	}
	root := fstree.TempRoot(t)

	rapid.Check(t, func(rt *rapid.T) {
		dir, err := os.MkdirTemp(root, "case-*")
		if err != nil {
			rt.Fatalf("mkdtemp: %v", err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		allowed := filepath.Join(dir, "allowed")
		inside := filepath.Join(allowed, "in")
		outside := filepath.Join(dir, "outside")
		outsideSub := filepath.Join(outside, "sub")
		for _, d := range []string{inside, outsideSub} {
			if err = os.MkdirAll(d, 0o755); err != nil {
				rt.Fatalf("mkdir: %v", err)
			}
		}
		// A second link out of the root, for the two-link shape: read as text,
		// `nope/../toout/new` is `toout/new`, inside the root and through it.
		fstree.MustSymlink(rt, outsideSub, filepath.Join(allowed, "toout"))
		validator := virtualPathValidator{resolver: virtualPathResolver{
			allowedRoots: normalizedRoots([]string{allowed}),
			access:       invowkfile.VirtualFilesystemAccessRestricted,
		}}

		shape := rapid.SampledFrom(shapes).Draw(rt, "link")
		targetInside := rapid.Bool().Draw(rt, "targetInside")
		absolute := rapid.Bool().Draw(rt, "absoluteTarget")
		useLink := shape != propNoLink && rapid.Bool().Draw(rt, "useLink")
		missing := rapid.IntRange(0, 3).Draw(rt, "missing")

		// T, as components relative to the allowed root, and as a path.
		targetParts, targetDir := []string{"..", "outside", "sub"}, outsideSub
		if targetInside {
			targetParts, targetDir = []string{"in"}, inside
		}
		last := targetParts[len(targetParts)-1]
		linkTarget := slices.Clone(targetParts)
		resolved := targetDir // where the link leads, when it can be resolved
		switch shape {
		case propParentOfExisting:
			linkTarget = append(linkTarget, "..", last)
		case propDangling:
			linkTarget = append(linkTarget, "gone")
			resolved = filepath.Join(targetDir, "gone")
		case propParentBeforeMissing:
			linkTarget = append(linkTarget, "..", last, "gone")
			resolved = filepath.Join(targetDir, "gone")
		case propParentAfterMissing:
			linkTarget = append(linkTarget, "nope", "..", "gone")
		case propTwoLinks:
			linkTarget = []string{"nope", "..", "toout", "new"}
		}
		// The two shapes that cannot be resolved hold a ".." after a missing
		// component. Windows may rewrite an absolute link target when it creates
		// the link, applying that ".." to the text, so there these two shapes
		// are written as relative targets only, which are stored as given.
		unresolvable := shape == propParentAfterMissing || shape == propTwoLinks
		if runtime.GOOS == goosWindows && unresolvable {
			absolute = false
		}
		link := filepath.Join(allowed, "x")
		// Targets are joined as text: filepath.Join would clean a ".." away.
		targetText := strings.Join(linkTarget, string(filepath.Separator))
		switch {
		case shape == propNoLink:
		case shape == propJunction:
			fstree.MustJunction(rt, targetDir, link)
		case absolute:
			fstree.MustSymlink(rt, allowed+string(filepath.Separator)+targetText, link)
		default:
			fstree.MustSymlink(rt, targetText, link)
		}

		// The oracle, from how the tree was built.
		class, want := propInside, allowed
		if useLink {
			switch {
			case unresolvable:
				class, want = propUnresolvable, ""
			case targetInside:
				want = resolved
			default:
				class, want = propOutside, ""
			}
		}
		reqPath := allowed
		if useLink {
			reqPath = link
		}
		for i := range missing {
			name := fmt.Sprintf("m%d", i+1)
			reqPath = filepath.Join(reqPath, name)
			if want != "" {
				want = filepath.Join(want, name)
			}
		}

		normalized, err := validator.validate(allowed, reqPath)
		if err == nil {
			// Do what a script does with an accepted path, so that an escape
			// leaves a trace outside.
			if mkErr := os.MkdirAll(normalized, 0o755); mkErr != nil {
				rt.Fatalf("mkdir -p on the accepted path %s: %v", normalized, mkErr)
			}
		}
		describe := fmt.Sprintf("%q (link %s, target inside=%v, absolute=%v, through the link=%v, %d missing)",
			reqPath, shape, targetInside, absolute, useLink, missing)
		switch class {
		case propInside:
			if err != nil {
				rt.Fatalf("false denial: %s resolves inside the root: %v", describe, err)
			}
			if normalized != want {
				rt.Fatalf("%s validated to %q, want %q", describe, normalized, want)
			}
			if physical, evalErr := filepath.EvalSymlinks(normalized); evalErr != nil || !fstree.Within(allowed, physical) {
				rt.Fatalf("%s was created at %q (%v), outside the root", describe, physical, evalErr)
			}
		case propOutside:
			if !errors.Is(err, errVirtualPathDenied) {
				rt.Fatalf("policy violation: %s resolves outside the root, got (%q, %v)", describe, normalized, err)
			}
		case propUnresolvable:
			if !errors.Is(err, errVirtualPathParentOfMissing) {
				rt.Fatalf("policy violation: %s cannot be resolved, got (%q, %v)", describe, normalized, err)
			}
		}
		// Nothing outside the root but what the test put there.
		if names := dirNames(rt, outside); !slices.Equal(names, []string{"sub"}) {
			rt.Fatalf("%s: the outside directory holds %q, want only \"sub\"", describe, names)
		}
		if names := dirNames(rt, outsideSub); len(names) != 0 {
			rt.Fatalf("%s: the link's outside target holds %q, want nothing", describe, names)
		}
	})
}

// dirNames lists the entries of dir.
func dirNames(rt *rapid.T, dir string) []string {
	rt.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		rt.Fatalf("read %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
