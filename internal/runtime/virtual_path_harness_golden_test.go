// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/internal/testutil/alloygolden"
	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/internal/testutil/mpctree"
	"github.com/invowk/invowk/pkg/invowkfile"
)

const (
	// virtualPathHarnessGoldenFile holds every instance of
	// formal/alloy/VirtualPathHarness.als's `golden` command.
	virtualPathHarnessGoldenFile = "testdata/formal/virtual_path_harness_golden.json.gz"

	// Link kinds of a harness request, as the model names them.
	harnessLinkNone     = ""
	harnessLinkSymlink  = "Symlink"
	harnessLinkJunction = "Junction"
)

// harnessAncestor identifies one materialised deepest existing ancestor.
type harnessAncestor struct {
	lex, phys, link string
}

// TestVirtualPathHarness_GoldenVectors replays every harness instance against a
// directly built virtualPathResolver over materialised roots (design D5.4/D5.5):
// the real validate decision under the current code must equal the model's
// dAcceptsCurrent. A request is materialised by its link kind: a plain
// directory, a symbolic link, or on Windows a junction, including links whose
// target stays in the same location. Junction requests are left out where the
// host has no junctions (everywhere but Windows, by the fstree capability
// policy) and counted in the log; the test itself is never skipped, because it
// is a killer. It fails if no instance places a node outside every root, and on
// a host with junctions if it replayed no junction request.
func TestVirtualPathHarness_GoldenVectors(t *testing.T) {
	t.Parallel()
	// Instances whose lexical and physical ancestors differ need a link.
	fstree.Require(t, fstree.CapSymlink)
	junctions := harnessJunctions(t)

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
	// Missing components are never created, so one ancestor per (lex, phys,
	// link kind) serves every request.
	ancestors := map[harnessAncestor]string{}
	locs := []string{"InReadRoot", "InWorkdir", "OutsideAll"}
	depths := []string{"Zero", "One", "TwoPlus"}

	sawOutside := false
	junctionsReplayed, junctionsLeftOut := 0, 0
	report := mpctree.NewReporter(t)
	for i, inst := range instances {
		accCurrent := inst.Targets("dAcceptsCurrent", inst.Atoms("Decisions")[0])
		for n, req := range inst.Atoms("Req") {
			key := harnessAncestor{
				lex:  mpctree.Which(inst, inst.Target("lexAnc", req), locs...),
				phys: mpctree.Which(inst, inst.Target("physAnc", req), locs...),
				link: harnessLinkKind(inst, req),
			}
			depth := mpctree.Which(inst, inst.Target("depth", req), depths...)
			if key.link == harnessLinkJunction {
				if !junctions {
					junctionsLeftOut++
					continue
				}
				junctionsReplayed++
			}
			sawOutside = sawOutside || key.phys == "OutsideAll"
			if _, ok := ancestors[key]; !ok {
				ancestors[key] = buildAncestor(t, dirFor[key.lex], dirFor[key.phys], key.link, key.lex+"_"+key.phys)
			}
			reqPath := requestPath(ancestors[key], depth)
			_, err := validator.validate(base, reqPath)
			if gotAccept, wantAccept := err == nil, slices.Contains(accCurrent, req); gotAccept != wantAccept {
				report.Errorf("instance %d req %d (lex=%s phys=%s link=%q depth=%s): real accept=%v, model dAcceptsCurrent=%v (err %v)",
					i, n, key.lex, key.phys, key.link, depth, gotAccept, wantAccept, err)
			}
		}
	}
	if !sawOutside {
		t.Fatal("no golden instance placed a node outside every allowed root; the check would be vacuous")
	}
	t.Logf("junction requests: %d replayed, %d left out (no junctions on %s)", junctionsReplayed, junctionsLeftOut, runtime.GOOS)
	if junctions && junctionsReplayed == 0 {
		t.Fatal("the host has junctions, but no junction request was replayed")
	}
	if junctionsReplayed+junctionsLeftOut == 0 {
		t.Fatal("no golden instance holds a junction request; the junction vectors would be vacuous")
	}
	report.Finish("VirtualPathHarness", len(instances))
}

// harnessJunctions reports whether the host can create junctions. Where the
// capability policy requires them (Windows) a failed probe fails the test;
// elsewhere the caller leaves the junction requests out and counts them. It
// never skips the test.
func harnessJunctions(t *testing.T) bool {
	t.Helper()
	ok, err := fstree.Probe(t).Has(fstree.CapJunction)
	if !ok && fstree.CapJunction.Required(fstree.CurrentHost()) {
		t.Fatalf("%s is required on %s: %v", fstree.CapJunction, runtime.GOOS, err)
	}
	return ok
}

// harnessLinkKind returns the model's link kind of a request, or
// harnessLinkNone when it crosses no link.
func harnessLinkKind(inst alloygolden.Instance, req string) string {
	link := inst.Target("link", req)
	if link == "" {
		return harnessLinkNone
	}
	return mpctree.Which(inst, link, harnessLinkSymlink, harnessLinkJunction)
}

// buildAncestor materialises a deepest existing ancestor that is lexically under
// lexDir and physically under physDir: a plain directory when the request
// crosses no link, otherwise a symlink or a junction under lexDir to a
// directory under physDir (the same location for a link that stays inside).
func buildAncestor(t *testing.T, lexDir, physDir, link, tag string) string {
	t.Helper()
	switch link {
	case harnessLinkNone:
		if lexDir != physDir {
			t.Fatalf("request %s moves between locations without a link; the model forbids it", tag)
		}
		return mustMkdir(t, filepath.Join(lexDir, "anc_"+tag))
	case harnessLinkSymlink:
		path := filepath.Join(lexDir, "lnk_"+tag)
		fstree.MustSymlink(t, mustMkdir(t, filepath.Join(physDir, "tgt_"+tag)), path)
		return path
	case harnessLinkJunction:
		path := filepath.Join(lexDir, "jun_"+tag)
		fstree.MustJunction(t, filepath.Join(physDir, "jtgt_"+tag), path)
		return path
	default:
		t.Fatalf("request %s has the unknown link kind %q", tag, link)
		return ""
	}
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
