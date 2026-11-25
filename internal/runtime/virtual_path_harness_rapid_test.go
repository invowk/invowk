// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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

	// Workdir sources and placements of the workdir property.
	propSourceCLI          = "cli override"
	propSourcePlain        = "invowkfile outside a module"
	propSourceDefault      = "module default"
	propSourceDeclared     = "module-declared"
	propPlaceInside        = "inside the module"
	propPlaceDotDot        = "outside by .."
	propPlaceAbsolute      = "absolute outside path"
	propPlaceLinkOut       = "link that leaves the module"
	propPlaceLinkIn        = "link that stays in the module"
	propPlaceJunctionOut   = "junction that leaves the module"
	propPlaceJunctionIn    = "junction that stays in the module"
	propWorkdirLevelRoot   = "root"
	propWorkdirLevelCmd    = "command"
	propWorkdirLevelImpl   = "implementation"
	propWorkdirLinkName    = "lnk"
	propWorkdirSharedName  = "shared"
	propWorkdirSubName     = "sub"
	propWorkdirModuleName  = "mod"
	propWorkdirMaxMissing  = 2
	propWorkdirRequestName = "m"
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
//
// A second property covers the workdir (rows workdirIsRoot and
// allowedTrustedWorkdir): it generates a module directory, a workdir source
// (CLI override, invowkfile outside a module, module default, module-declared
// at the root, command or implementation level) and a placement (inside the
// module, outside by "..", an absolute outside path, a link, on Windows also a
// junction, that leaves or stays in the module), builds the workdir input
// through virtualWorkdirFor, and compares the production decision and the
// validator's verdict on a request below the workdir with an oracle from how
// the tree was built: the workdir is a root when it is trusted or lies
// physically inside the module. The verdict half builds the roots directly
// (the module, and the workdir when the decision admits it), so no implicit
// root encloses the generated tree.
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
		targetText := rawLinkTarget(linkTarget...)
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

	checkWorkdirProperty(t, root)
}

// checkWorkdirProperty is the workdir half of TestVirtualPathHarness_Property.
func checkWorkdirProperty(t *testing.T, root string) {
	t.Helper()
	placements := []string{propPlaceInside, propPlaceDotDot, propPlaceAbsolute, propPlaceLinkOut, propPlaceLinkIn}
	if harnessJunctions(t) {
		placements = append(placements, propPlaceJunctionOut, propPlaceJunctionIn)
	}
	sources := []string{propSourceCLI, propSourcePlain, propSourceDefault, propSourceDeclared}
	levels := []string{propWorkdirLevelRoot, propWorkdirLevelCmd, propWorkdirLevelImpl}

	rapid.Check(t, func(rt *rapid.T) {
		dir, err := os.MkdirTemp(root, "workdir-*")
		if err != nil {
			rt.Fatalf("mkdtemp: %v", err)
		}
		defer func() { _ = os.RemoveAll(dir) }()
		module := filepath.Join(dir, propWorkdirModuleName)
		sub := filepath.Join(module, propWorkdirSubName)
		shared := filepath.Join(dir, propWorkdirSharedName)
		for _, d := range []string{sub, shared} {
			if err = os.MkdirAll(d, 0o755); err != nil {
				rt.Fatalf("mkdir: %v", err)
			}
		}

		source := rapid.SampledFrom(sources).Draw(rt, "source")
		placement := rapid.SampledFrom(placements).Draw(rt, "placement")
		level := rapid.SampledFrom(levels).Draw(rt, "level")
		missing := rapid.IntRange(0, propWorkdirMaxMissing).Draw(rt, "missing")

		// The workdir value and where it physically lies, from the tree.
		link := filepath.Join(module, propWorkdirLinkName)
		value, inside := "", false
		switch placement {
		case propPlaceInside:
			value, inside = propWorkdirSubName, true
		case propPlaceDotDot:
			value = "../" + propWorkdirSharedName
		case propPlaceAbsolute:
			value = shared
		case propPlaceLinkOut:
			fstree.MustSymlink(rt, shared, link)
			value = propWorkdirLinkName
		case propPlaceLinkIn:
			fstree.MustSymlink(rt, sub, link)
			value, inside = propWorkdirLinkName, true
		case propPlaceJunctionOut:
			fstree.MustJunction(rt, shared, link)
			value = propWorkdirLinkName
		case propPlaceJunctionIn:
			fstree.MustJunction(rt, sub, link)
			value, inside = propWorkdirLinkName, true
		}

		inv := &invowkfile.Invowkfile{FilePath: invowkfile.FilesystemPath(filepath.Join(module, "invowkfile.cue"))}
		if source != propSourcePlain {
			inv.ModulePath = invowkfile.FilesystemPath(module)
		}
		cmd := &invowkfile.Command{Name: "probe"}
		impl := &invowkfile.Implementation{Script: invowkfile.ImplementationScript{Content: "echo"}}
		var override invowkfile.WorkDir
		switch {
		case source == propSourceDefault:
			inside = true
		case source == propSourceCLI:
			override = invowkfile.WorkDir(value)
		case level == propWorkdirLevelRoot:
			inv.WorkDir = invowkfile.WorkDir(value)
		case level == propWorkdirLevelCmd:
			cmd.WorkDir = invowkfile.WorkDir(value)
		default:
			impl.WorkDir = invowkfile.WorkDir(value)
		}
		// The oracle: a workdir is a root when it is trusted or inside.
		declared := source == propSourceDeclared
		wantRoot := !declared || inside

		workdir := virtualWorkdirFor(inv, cmd, impl, override)
		describe := fmt.Sprintf("workdir %q (source %s, level %s, placement %s, %d missing)",
			workdir.path, source, level, placement, missing)
		if got := workdir.moduleRoot != ""; got != declared {
			rt.Fatalf("%s: module root %q, want recorded=%v", describe, workdir.moduleRoot, declared)
		}
		isRoot := workdir.isReadRoot()
		if isRoot != wantRoot {
			rt.Fatalf("%s: isReadRoot() = %v, oracle %v", describe, isRoot, wantRoot)
		}

		roots := []string{module}
		if isRoot {
			roots = append(roots, workdir.path)
		}
		validator := pathTestValidator(invowkfile.VirtualFilesystemAccessRestricted, roots...)
		request := workdir.path
		for i := range missing {
			request = filepath.Join(request, fmt.Sprintf("%s%d", propWorkdirRequestName, i+1))
		}
		_, err = validator.validate(module, request)
		switch {
		case wantRoot && err != nil:
			rt.Fatalf("false denial: %s is an allowed root, the request %q was denied: %v", describe, request, err)
		case !wantRoot && !errors.Is(err, errVirtualPathDenied):
			rt.Fatalf("policy violation: %s lies outside the module, the request %q got %v", describe, request, err)
		}
	})
}

// dirNames lists the entries of dir, in name order.
func dirNames(t fstree.Fataler, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	return names
}
