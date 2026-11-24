// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// noFollowHarnessCase is one no-follow request of
// TestVirtualPathHarness_NoFollowOperands: the request, the path the
// no-follow form must give (the parent's physical location joined with the
// final name, from how the test built the tree), and whether that path lies
// inside the allowed root.
type noFollowHarnessCase struct {
	name    string
	request string
	want    string
	inside  bool
}

// TestVirtualPathHarness_NoFollowOperands binds the no-follow rows of the
// VirtualPathHarness correspondence table (resolveNoFollow, validateNoFollow;
// design D7 of virtual-path-no-follow-operands). A no-follow request's parent
// is a model request, resolved by the same walk, and its final name is joined
// unread. The test builds the trees with fstree, computes the expected path
// and verdict from how it built them, and compares validateNoFollow's path and
// verdict under both access modes: under restricted access the request is
// accepted exactly when the expected path lies inside the root, and under full
// access it is always accepted with that path. The cases are a live link, a
// dangling link, a link in a cycle, a link inside the root to an outside
// file, a parent reached through an escaping link, a missing name below an
// existing directory, a final name equal to the root, and a link to the root
// (whose own path lies outside it). The junction cases are a subtest, the only
// place that calls fstree.Require for junctions, so this killer itself never
// skips.
func TestVirtualPathHarness_NoFollowOperands(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	base := fstree.TempRoot(t)
	root := mustMkdir(t, filepath.Join(base, "root"))
	outside := mustMkdir(t, filepath.Join(base, "outside"))
	mustWriteTestFile(t, filepath.Join(root, "target.txt"), "target")
	mustWriteTestFile(t, filepath.Join(outside, "outfile"), "outside")
	mustWriteTestFile(t, filepath.Join(outside, "name"), "outside")
	mustMkdir(t, filepath.Join(root, "sub"))
	fstree.MustSymlink(t, "target.txt", filepath.Join(root, "live"))
	fstree.MustSymlink(t, "missing.txt", filepath.Join(root, "stale"))
	fstree.MustSymlink(t, "cyc", filepath.Join(root, "cyc"))
	fstree.MustSymlink(t, filepath.Join(outside, "outfile"), filepath.Join(root, "outlink"))
	fstree.MustSymlink(t, outside, filepath.Join(root, "esc"))
	fstree.MustSymlink(t, root, filepath.Join(base, "rootlink"))

	cases := []noFollowHarnessCase{
		{name: "live link", request: "live", want: filepath.Join(root, "live"), inside: true},
		{name: "dangling link", request: "stale", want: filepath.Join(root, "stale"), inside: true},
		{name: "link in a cycle", request: "cyc", want: filepath.Join(root, "cyc"), inside: true},
		{name: "link to an outside file", request: "outlink", want: filepath.Join(root, "outlink"), inside: true},
		{name: "parent through an escaping link", request: filepath.Join("esc", "name"), want: filepath.Join(outside, "name")},
		{name: "missing name", request: filepath.Join("sub", "new"), want: filepath.Join(root, "sub", "new"), inside: true},
		{name: "final name equal to the root", request: root, want: root, inside: true},
		{name: "link to the root", request: filepath.Join(base, "rootlink"), want: filepath.Join(base, "rootlink")},
	}
	checkNoFollowHarnessCases(t, root, cases)

	t.Run("junction", func(t *testing.T) {
		t.Parallel()
		fstree.Require(t, fstree.CapJunction)
		jbase := fstree.TempRoot(t)
		jroot := mustMkdir(t, filepath.Join(jbase, "root"))
		jout := mustMkdir(t, filepath.Join(jbase, "outside"))
		mustWriteTestFile(t, filepath.Join(jout, "f"), "outside")
		fstree.MustJunction(t, jout, filepath.Join(jroot, "j"))
		checkNoFollowHarnessCases(t, jroot, []noFollowHarnessCase{
			{name: "junction to an outside directory", request: "j", want: filepath.Join(jroot, "j"), inside: true},
			{name: "parent through the junction", request: filepath.Join("j", "f"), want: filepath.Join(jout, "f")},
		})
	})
}

// checkNoFollowHarnessCases compares validateNoFollow, run from root, with
// each case under restricted and full access.
func checkNoFollowHarnessCases(t *testing.T, root string, cases []noFollowHarnessCase) {
	t.Helper()
	for _, access := range virtualPathBothAccessModes {
		validator := virtualPathValidator{resolver: virtualPathResolver{
			allowedRoots: normalizedRoots([]string{root}),
			access:       access,
		}}
		for _, tc := range cases {
			got, err := validator.validateNoFollow(root, tc.request)
			accept := tc.inside || access == invowkfile.VirtualFilesystemAccessFull
			switch {
			case accept && (err != nil || got != tc.want):
				t.Errorf("%s/%s: validateNoFollow(%s) = (%q, %v), want (%q, nil)", access, tc.name, tc.request, got, err, tc.want)
			case !accept && (!errors.Is(err, errVirtualPathDenied) || !strings.Contains(err.Error(), tc.want)):
				t.Errorf("%s/%s: validateNoFollow(%s) = (%q, %v), want the denial naming %s", access, tc.name, tc.request, got, err, tc.want)
			}
		}
	}
}
