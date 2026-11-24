// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
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

	// harnessRequestShapes is the number of distinct requests the model
	// allows: 3 depths x 9 (lexical, physical) location pairs x the link
	// kinds a pair admits (2 when the locations differ, 3 when they agree) is
	// 63, plus the dangling requests, 2 depths x 9 pairs x Symlink, 18.
	harnessRequestShapes = 81
)

// harnessAncestor identifies one materialised request ancestor: the deepest
// existing ancestor, or for a dangling request the link whose target is
// missing.
type harnessAncestor struct {
	lex, phys, link string
	dangling        bool
}

// TestVirtualPathHarness_GoldenVectors replays every harness instance against a
// directly built virtualPathResolver over materialised roots (design D5.4/D5.5):
// the real validate decision must equal the model's fixed-evaluation decision
// dAccepts on every request, with no finding carve-out. A request is
// materialised by its link kind: a plain directory, a symbolic link, on
// Windows a junction, and for a dangling request a symbolic link to a missing
// name, including links whose target stays in the same location.
//
// Junction requests are left out where the host has no junctions (everywhere
// but Windows, by the fstree capability policy) and counted in the log; the
// test itself is never skipped, because it is a killer. It fails if no instance
// places a node outside every root, on a host with junctions if it replayed no
// junction request, and without -short if the golden does not hold every
// request shape of the model.
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
	// The code still adds the workdir to the allowed roots (F15 is open;
	// H.includeWorkdir is fixed True in the golden). Build the resolver directly
	// so t.TempDir's own enclosing roots (/tmp, HOME) do not make the check
	// vacuous (design D5.4).
	validator := virtualPathValidator{resolver: virtualPathResolver{
		allowedRoots: normalizedRoots([]string{dirFor["InReadRoot"], dirFor["InWorkdir"]}),
		access:       invowkfile.VirtualFilesystemAccessRestricted,
	}}
	expectHarnessResolutionErrors(t, validator, dirFor["InReadRoot"])
	// Missing components are never created, so one ancestor per (lex, phys,
	// link kind, dangling) serves every request.
	ancestors := map[harnessAncestor]string{}
	locs := []string{"InReadRoot", "InWorkdir", "OutsideAll"}
	depths := []string{"Zero", "One", "TwoPlus"}

	sawOutside := false
	junctionsReplayed, junctionsLeftOut := 0, 0
	shapes := map[string]bool{}
	report := mpctree.NewReporter(t)
	for i, inst := range instances {
		accepts := inst.Targets("dAccepts", inst.Atoms("Decisions")[0])
		for n, req := range inst.Atoms("Req") {
			key := harnessAncestor{
				lex:      mpctree.Which(inst, inst.Target("lexAnc", req), locs...),
				phys:     mpctree.Which(inst, inst.Target("physAnc", req), locs...),
				link:     harnessLinkKind(inst, req),
				dangling: mpctree.Which(inst, inst.Target("dangling", req), "True", "False") == "True",
			}
			depth := mpctree.Which(inst, inst.Target("depth", req), depths...)
			shapes[fmt.Sprintf("%+v depth=%s", key, depth)] = true
			if key.link == harnessLinkJunction {
				if !junctions {
					junctionsLeftOut++
					continue
				}
				junctionsReplayed++
			}
			sawOutside = sawOutside || key.phys == "OutsideAll"
			if _, ok := ancestors[key]; !ok {
				ancestors[key] = buildAncestor(t, dirFor[key.lex], dirFor[key.phys], key)
			}
			reqPath := requestPath(ancestors[key], depth, key.dangling)
			_, err := validator.validate(base, reqPath)
			if gotAccept, wantAccept := err == nil, slices.Contains(accepts, req); gotAccept != wantAccept {
				report.Errorf("instance %d req %d (lex=%s phys=%s link=%q depth=%s dangling=%v): real accept=%v, model dAccepts=%v (err %v)",
					i, n, key.lex, key.phys, key.link, depth, key.dangling, gotAccept, wantAccept, err)
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
	// Two requests per instance must still show every request shape (under
	// -short only every 16th instance is loaded).
	if !testing.Short() && len(shapes) != harnessRequestShapes {
		t.Fatalf("the golden holds %d request shapes, want %d", len(shapes), harnessRequestShapes)
	}
	report.Finish("VirtualPathHarness", len(instances))
}

// expectHarnessResolutionErrors checks what the model does not hold: a path the
// resolver cannot resolve has no request in the model, and the validator
// refuses it with the resolution error itself, which names the path, never
// with a path denial and never by comparing an unresolved path. An empty path
// is an error too.
func expectHarnessResolutionErrors(t *testing.T, validator virtualPathValidator, readRoot string) {
	t.Helper()
	if normalized, err := validator.validate(readRoot, ""); err == nil || errors.Is(err, errVirtualPathDenied) {
		t.Errorf("validate of an empty path = (%q, %v), want a resolution error", normalized, err)
	}
	fstree.MustSymlink(t, "cycle", filepath.Join(readRoot, "cycle"))
	request := filepath.Join(readRoot, "cycle", "m1")
	normalized, err := validator.validate(readRoot, request)
	if !errors.Is(err, errVirtualPathTooManyLinks) || errors.Is(err, errVirtualPathDenied) {
		t.Fatalf("validate through a link cycle = (%q, %v), want errVirtualPathTooManyLinks", normalized, err)
	}
	if !strings.Contains(err.Error(), request) {
		t.Errorf("the resolution error does not name the path %s: %v", request, err)
	}
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

// buildAncestor materialises a request's ancestor lexically under lexDir and
// physically under physDir: a plain directory when the request crosses no
// link, otherwise a symlink or a junction under lexDir to a directory under
// physDir (the same location for a link that stays inside). For a dangling
// request the symlink points at a missing name in a directory under physDir,
// so that directory is the deepest existing ancestor.
func buildAncestor(t *testing.T, lexDir, physDir string, key harnessAncestor) string {
	t.Helper()
	tag := key.lex + "_" + key.phys
	switch {
	case key.dangling:
		if key.link != harnessLinkSymlink {
			t.Fatalf("dangling request %s has the link kind %q; the model allows only a symlink", tag, key.link)
		}
		path := filepath.Join(lexDir, "dlnk_"+tag)
		fstree.MustSymlink(t, filepath.Join(mustMkdir(t, filepath.Join(physDir, "dtgt_"+tag)), "missing"), path)
		return path
	case key.link == harnessLinkNone:
		if lexDir != physDir {
			t.Fatalf("request %s moves between locations without a link; the model forbids it", tag)
		}
		return mustMkdir(t, filepath.Join(lexDir, "anc_"+tag))
	case key.link == harnessLinkSymlink:
		path := filepath.Join(lexDir, "lnk_"+tag)
		fstree.MustSymlink(t, mustMkdir(t, filepath.Join(physDir, "tgt_"+tag)), path)
		return path
	case key.link == harnessLinkJunction:
		path := filepath.Join(lexDir, "jun_"+tag)
		fstree.MustJunction(t, filepath.Join(physDir, "jtgt_"+tag), path)
		return path
	default:
		t.Fatalf("request %s has the unknown link kind %q", tag, key.link)
		return ""
	}
}

// requestPath appends the model's number of missing components. For a
// dangling request the ancestor is the link, whose missing target is the
// first missing component: the request is the link at depth One, and the link
// plus one missing name at TwoPlus.
func requestPath(ancestor, depth string, dangling bool) string {
	missing := map[string]int{"Zero": 0, "One": 1, "TwoPlus": 2}[depth]
	if dangling {
		missing--
	}
	path := ancestor
	for i := range missing {
		path = filepath.Join(path, fmt.Sprintf("m%d", i+1))
	}
	return path
}

func mustMkdir(t *testing.T, dir string) string {
	t.Helper()
	testutil.MustMkdirAll(t, dir, 0o755)
	return dir
}
