// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/internal/testutil/alloygolden"
	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/internal/testutil/mpctree"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// virtualPathHarnessGoldenFile holds every instance of
// formal/alloy/VirtualPathHarness.als's `golden` command.
const virtualPathHarnessGoldenFile = "testdata/formal/virtual_path_harness_golden.json.gz"

// TestVirtualPathHarness_GoldenVectors replays every harness instance against a
// directly built virtualPathResolver over materialised roots (design D5.4/D5.5):
// the real validate decision under the current code must equal the model's
// dAcceptsCurrent. It fails if no instance places a node outside every root.
func TestVirtualPathHarness_GoldenVectors(t *testing.T) {
	t.Parallel()

	instances := alloygolden.Load(t, virtualPathHarnessGoldenFile, "VirtualPathHarness")

	base := fstree.TempRoot(t)
	dirFor := map[string]string{
		"InReadRoot": mustMkdir(t, filepath.Join(base, "read")),
		"InWorkdir":  mustMkdir(t, filepath.Join(base, "work")),
		"OutsideAll": mustMkdir(t, filepath.Join(base, "outside")),
	}
	// The current code adds the workdir to the allowed roots (H.includeWorkdir is
	// fixed True in the golden). Build the resolver directly so t.TempDir's own
	// enclosing roots (/tmp, HOME) do not make the check vacuous (design D5.4).
	validator := virtualPathValidator{resolver: virtualPathResolver{
		allowedRoots: normalizedRoots([]string{dirFor["InReadRoot"], dirFor["InWorkdir"]}),
		access:       invowkfile.VirtualFilesystemAccessRestricted,
	}}
	// Missing components are never created, so one ancestor per (lex, phys) pair
	// serves every request.
	ancestors := map[[2]string]string{}
	locs := []string{"InReadRoot", "InWorkdir", "OutsideAll"}
	depths := []string{"Zero", "One", "TwoPlus"}

	sawOutside := false
	report := mpctree.NewReporter(t)
	for i, inst := range instances {
		accCurrent := inst.Targets("dAcceptsCurrent", inst.Atoms("Decisions")[0])
		for n, req := range inst.Atoms("Req") {
			lex := mpctree.Which(inst, inst.Target("lexAnc", req), locs...)
			phys := mpctree.Which(inst, inst.Target("physAnc", req), locs...)
			depth := mpctree.Which(inst, inst.Target("depth", req), depths...)
			sawOutside = sawOutside || phys == "OutsideAll"
			pair := [2]string{lex, phys}
			if _, ok := ancestors[pair]; !ok {
				ancestors[pair] = buildAncestor(t, dirFor[lex], dirFor[phys], lex+"_"+phys)
			}
			reqPath := requestPath(ancestors[pair], depth)
			_, err := validator.validate(base, reqPath)
			if gotAccept, wantAccept := err == nil, slices.Contains(accCurrent, req); gotAccept != wantAccept {
				report.Errorf("instance %d req %d (lex=%s phys=%s depth=%s): real accept=%v, model dAcceptsCurrent=%v",
					i, n, lex, phys, depth, gotAccept, wantAccept)
			}
		}
	}
	if !sawOutside {
		t.Fatal("no golden instance placed a node outside every allowed root; the check would be vacuous")
	}
	report.Finish("VirtualPathHarness", len(instances))
}

// buildAncestor materialises a deepest existing ancestor that is lexically under
// lexDir and physically under physDir: a plain directory when they agree,
// otherwise a symlink under lexDir to a directory under physDir.
func buildAncestor(t *testing.T, lexDir, physDir, tag string) string {
	t.Helper()
	if lexDir == physDir {
		return mustMkdir(t, filepath.Join(lexDir, "anc_"+tag))
	}
	link := filepath.Join(lexDir, "lnk_"+tag)
	if err := os.Symlink(mustMkdir(t, filepath.Join(physDir, "tgt_"+tag)), link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	return link
}

// requestPath appends the model's number of missing components.
func requestPath(ancestor, depth string) string {
	switch depth {
	case "Zero":
		return ancestor
	case "One":
		return filepath.Join(ancestor, "m1")
	default: // TwoPlus
		return filepath.Join(ancestor, "m1", "m2")
	}
}

func mustMkdir(t *testing.T, dir string) string {
	t.Helper()
	testutil.MustMkdirAll(t, dir, 0o755)
	return dir
}
