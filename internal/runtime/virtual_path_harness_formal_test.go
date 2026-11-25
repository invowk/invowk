// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkfile"
)

type (
	// harnessWorkdirKind is one way a command run gets its workdir in the
	// construction replay: the invowkfile, command, implementation and CLI
	// override it is built from, and the model's workdir kind.
	harnessWorkdirKind struct {
		name     string
		inv      *invowkfile.Invowkfile
		cmd      *invowkfile.Command
		impl     *invowkfile.Implementation
		override invowkfile.WorkDir
		// trusted: H.workdir = TrustedWorkdir; otherwise ModuleOutside, or
		// ModuleInside when enclosed is set.
		trusted bool
		// enclosed: the workdir lies in the script base, so it has no
		// workdir-only location (the model's "no InWorkdir").
		enclosed bool
		// symlink: the kind needs a symbolic link.
		symlink bool
	}

	// rootConstructionProbes holds the paths of one construction replay.
	rootConstructionProbes struct {
		scriptBase, scriptFile, workDir, declared, outside, tag string
	}
)

// TestVirtualPathHarness_DeepMissingPathThroughSymlinkDenied replays the fixed
// finding F14: the pre-fix normalizeExistingOrParent evaluated only the path
// and its parent, so a path with two or more missing components below a
// symlink that leaves the root was judged by its text, accepted, and written
// outside. The resolver now finds the deepest existing ancestor physically, so
// the one-level and the two-level path are both denied and the outside
// directory stays empty.
func TestVirtualPathHarness_DeepMissingPathThroughSymlinkDenied(t *testing.T) {
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

	expectHarnessWriteDenied(t, validator, root, filepath.Join(root, "x", "a"))
	expectHarnessWriteDenied(t, validator, root, filepath.Join(root, "x", "a", "b"))
	expectHarnessWriteDenied(t, validator, root, filepath.Join(root, "x", "a", "b", "c"))
	expectHarnessDirEntries(t, outside)
}

// TestVirtualPathHarness_DanglingLinkEscapeDenied replays the fixed finding
// F19: the pre-fix normalizeExistingOrParent could not evaluate a symlink whose
// target is missing, fell back to the link's parent, and so judged the link as
// a missing plain name inside the root; the write then followed the link and
// created its target outside. It fails against the product code of c2fe96ff
// (formal/README.md, F19, has the steps). Each leg has a relative and an
// absolute target:
//   - a link to a missing file in an outside directory;
//   - a link to a missing file two missing levels below an outside directory;
//   - a link whose target is `nope/../in/new`, with `nope` missing and `in` a
//     link to the outside directory. Applied to the target's text, the ".."
//     gives `in/new`, inside the root and through a link nobody walked; the
//     resolver refuses a ".." after a missing component.
//
// Each is denied, and the outside directory is empty afterwards.
func TestVirtualPathHarness_DanglingLinkEscapeDenied(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	sep := string(filepath.Separator)
	for _, absolute := range []bool{false, true} {
		name := "relative target"
		if absolute {
			name = "absolute target"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			base := fstree.TempRoot(t)
			root := mustMkdir(t, filepath.Join(base, "root"))
			outside := mustMkdir(t, filepath.Join(base, "outside"))
			// A target as it is written into the link: never cleaned, so the
			// third leg keeps its "..".
			target := func(fromRoot ...string) string {
				if absolute {
					return root + sep + strings.Join(fromRoot, sep)
				}
				return strings.Join(fromRoot, sep)
			}
			fstree.MustSymlink(t, target("..", "outside"), filepath.Join(root, "in"))
			fstree.MustSymlink(t, target("..", "outside", "new1"), filepath.Join(root, "d1"))
			fstree.MustSymlink(t, target("..", "outside", "a", "b", "new2"), filepath.Join(root, "d2"))
			fstree.MustSymlink(t, target("nope", "..", "in", "new3"), filepath.Join(root, "d3"))
			validator := virtualPathValidator{resolver: virtualPathResolver{
				allowedRoots: normalizedRoots([]string{root}),
				access:       invowkfile.VirtualFilesystemAccessRestricted,
			}}

			for _, link := range []string{"d1", "d2", "d3"} {
				expectHarnessWriteDenied(t, validator, root, filepath.Join(root, link))
			}
			expectHarnessDirEntries(t, outside)
		})
	}
}

// TestVirtualPathHarness_JunctionEscapeDenied replays the fixed finding F18 on
// Windows: filepath.EvalSymlinks does not follow a junction, so the pre-fix
// validator judged a path through a junction by the junction's own location
// and admitted it (windows-latest, CI run 44523520503, job 135557989975). The
// walk follows junctions, so an existing file, a new file, and a new file two
// missing components below a junction to an outside directory are denied and
// nothing is created outside. A junction whose target is inside the root, and
// a root that is itself given through a junction, keep working at every depth.
//
// It is required on Windows and skipped elsewhere by the capability policy, so
// it is never a killer (the mutantPreFixF18JunctionNotFollowed row).
func TestVirtualPathHarness_JunctionEscapeDenied(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapJunction)
	base := fstree.TempRoot(t)
	root := mustMkdir(t, filepath.Join(base, "root"))
	inside := mustMkdir(t, filepath.Join(root, "inside"))
	outside := mustMkdir(t, filepath.Join(base, "outside"))
	sentinel := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(sentinel, []byte("outside"), 0o600); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	fstree.MustJunction(t, outside, filepath.Join(root, "j"))
	fstree.MustJunction(t, inside, filepath.Join(root, "in"))
	validator := virtualPathValidator{resolver: virtualPathResolver{
		allowedRoots: normalizedRoots([]string{root}),
		access:       invowkfile.VirtualFilesystemAccessRestricted,
	}}

	// An existing file below the junction: denied, so it cannot be read.
	if normalized, err := validator.validate(root, filepath.Join(root, "j", "secret.txt")); err == nil {
		content, readErr := os.ReadFile(normalized)
		t.Errorf("existing file through the junction accepted as %q (read %q, err %v)", normalized, content, readErr)
	}
	// A new file directly below the junction: denied, so it cannot be created.
	expectHarnessWriteDenied(t, validator, root, filepath.Join(root, "j", "new.txt"))
	// A new file two missing components below the junction: F14's shape through
	// a junction, also denied.
	expectHarnessWriteDenied(t, validator, root, filepath.Join(root, "j", "a", "b", "new.txt"))
	expectHarnessDirEntries(t, outside, "secret.txt")

	// A junction whose target is inside the root is followed and accepted.
	for path, want := range map[string]string{
		filepath.Join(root, "in"):                      inside,
		filepath.Join(root, "in", "new.txt"):           filepath.Join(inside, "new.txt"),
		filepath.Join(root, "in", "a", "b", "new.txt"): filepath.Join(inside, "a", "b", "new.txt"),
	} {
		if got, err := validator.validate(root, path); err != nil || got != want {
			t.Errorf("path through the in-root junction: validate(%s) = (%q, %v), want %q", path, got, err, want)
		}
	}

	// A root given through a junction is an allowed root, and a request
	// written through that junction is accepted.
	realDir := mustMkdir(t, filepath.Join(base, "real"))
	alias := filepath.Join(base, "alias")
	fstree.MustJunction(t, realDir, alias)
	aliased := virtualPathValidator{resolver: virtualPathResolver{
		allowedRoots: normalizedRoots([]string{alias}),
		access:       invowkfile.VirtualFilesystemAccessRestricted,
	}}
	if len(aliased.resolver.allowedRoots) != 1 {
		t.Fatalf("a root given through a junction was dropped: %v", aliased.resolver.allowedRoots)
	}
	for path, want := range map[string]string{
		alias:                                realDir,
		filepath.Join(alias, "new.txt"):      filepath.Join(realDir, "new.txt"),
		filepath.Join(alias, "new1", "new2"): filepath.Join(realDir, "new1", "new2"),
	} {
		if got, err := aliased.validate(alias, path); err != nil || got != want {
			t.Errorf("request through the root's junction: validate(%s) = (%q, %v), want %q", path, got, err, want)
		}
	}
	if _, err := aliased.validate(alias, filepath.Join(root, "new.txt")); err == nil {
		t.Error("a path outside the junctioned root was accepted")
	}
}

// expectHarnessWriteDenied fails when the validator accepts path. An accepted
// path is then written through, as a script would, so that the caller's check
// of the outside directory shows where the write landed.
func expectHarnessWriteDenied(t *testing.T, validator virtualPathValidator, cwd, path string) {
	t.Helper()
	normalized, err := validator.validate(cwd, path)
	if err == nil {
		t.Errorf("%s accepted as %s", path, normalized)
		if mkErr := os.MkdirAll(filepath.Dir(normalized), 0o755); mkErr != nil {
			t.Logf("mkdir for the accepted path: %v", mkErr)
		}
		if writeErr := os.WriteFile(normalized, []byte("escaped"), 0o600); writeErr != nil {
			t.Logf("write through the accepted path: %v", writeErr)
		}
	}
}

// expectHarnessDirEntries fails unless dir holds exactly the named entries.
func expectHarnessDirEntries(t *testing.T, dir string, want ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	got := make([]string, 0, len(entries))
	for _, entry := range entries {
		got = append(got, entry.Name())
	}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("%s holds %q, want %q", dir, got, want)
	}
}

// TestVirtualPathHarness_ModuleWorkdirOutsideNotRoot replays the fixed finding
// F15: a module-declared workdir outside the module widened the allowed roots
// under restricted access, so an access below it was accepted although a
// workdir is a cwd, not a read root. With the filesystem root ("/", or the
// volume root on Windows) declared as a module implementation's workdir,
// neither the workdir nor @work is an allowed root, @work still reports it,
// and a read below it that no other root encloses is denied. The same
// directory given as the CLI override is a root, and the read is accepted.
func TestVirtualPathHarness_ModuleWorkdirOutsideNotRoot(t *testing.T) {
	t.Parallel()
	base := fstree.TempRoot(t)
	module := mustMkdir(t, filepath.Join(base, "module"))
	fsRoot := filepath.VolumeName(base) + string(filepath.Separator)
	probe := filepath.Join(fsRoot, "etc", "hostname")
	if runtime.GOOS == goosWindows {
		probe = filepath.Join(fsRoot, "invowk-f15-probe-"+filepath.Base(base), "data.txt")
	}
	inv := &invowkfile.Invowkfile{
		FilePath:   invowkfile.FilesystemPath(filepath.Join(module, "invowkfile.cue")),
		ModulePath: invowkfile.FilesystemPath(module),
	}
	impl := &invowkfile.Implementation{
		Script:  invowkfile.ImplementationScript{Content: "echo"},
		WorkDir: invowkfile.WorkDir(fsRoot),
	}
	restricted := invowkfile.VirtualFilesystemConfig{Access: invowkfile.VirtualFilesystemAccessRestricted}

	declared := virtualWorkdirFor(inv, nil, impl, "")
	resolver, err := newVirtualPathResolverForFilesystem(declared, module, restricted)
	if err != nil {
		t.Fatalf("build resolver: %v", err)
	}
	if got := resolver.anchors[virtualAnchorWork]; got != fsRoot {
		t.Fatalf("@work = %q, want the workdir %q", got, fsRoot)
	}
	if slices.Contains(resolver.allowedRoots, fsRoot) {
		t.Fatalf("the module-declared workdir %q outside the module is an allowed root: %v", fsRoot, resolver.allowedRoots)
	}
	validator := virtualPathValidator{resolver: resolver}
	if normalized, denyErr := validator.validate(module, probe); !errors.Is(denyErr, errVirtualPathDenied) {
		t.Fatalf("read of %s below the module-declared workdir = (%q, %v), want virtual path denied", probe, normalized, denyErr)
	}

	override := virtualWorkdirFor(inv, nil, impl, invowkfile.WorkDir(fsRoot))
	resolver, err = newVirtualPathResolverForFilesystem(override, module, restricted)
	if err != nil {
		t.Fatalf("build resolver: %v", err)
	}
	if !slices.Contains(resolver.allowedRoots, fsRoot) {
		t.Fatalf("the CLI override %q is not an allowed root: %v", fsRoot, resolver.allowedRoots)
	}
	validator = virtualPathValidator{resolver: resolver}
	if _, err := validator.validate(module, probe); err != nil {
		t.Fatalf("read of %s below the CLI override was denied: %v", probe, err)
	}
}

// harnessWorkdirKinds returns every workdir kind of the construction replay.
// workDir is the outside directory, a missing directory under the volume
// root; scriptBase is the module directory (and the plain invowkfile's).
func harnessWorkdirKinds(scriptBase, workDir string) []harnessWorkdirKind {
	module := &invowkfile.Invowkfile{
		FilePath:   invowkfile.FilesystemPath(filepath.Join(scriptBase, "invowkfile.cue")),
		ModulePath: invowkfile.FilesystemPath(scriptBase),
	}
	plain := &invowkfile.Invowkfile{FilePath: module.FilePath}
	withRoot := func(inv *invowkfile.Invowkfile, workdir string) *invowkfile.Invowkfile {
		clone := *inv
		clone.WorkDir = invowkfile.WorkDir(filepath.ToSlash(workdir))
		return &clone
	}
	cmd := func(workdir string) *invowkfile.Command {
		return &invowkfile.Command{Name: "probe", WorkDir: invowkfile.WorkDir(filepath.ToSlash(workdir))}
	}
	impl := func(workdir string) *invowkfile.Implementation {
		return &invowkfile.Implementation{
			Script:  invowkfile.ImplementationScript{Content: "echo"},
			WorkDir: invowkfile.WorkDir(filepath.ToSlash(workdir)),
		}
	}
	dotDot, err := filepath.Rel(scriptBase, workDir)
	if err != nil {
		dotDot = workDir
	}
	kinds := []harnessWorkdirKind{
		{name: "cli override", inv: module, override: invowkfile.WorkDir(workDir), trusted: true},
		{name: "plain root", inv: withRoot(plain, dotDot), trusted: true},
		{name: "plain command", inv: plain, cmd: cmd(workDir), trusted: true},
		{name: "plain implementation", inv: plain, impl: impl(dotDot), trusted: true},
		{name: "module default", inv: module, trusted: true, enclosed: true},
		{name: "module inside", inv: module, cmd: cmd("sub"), enclosed: true},
		{name: "module root dot-dot", inv: withRoot(module, dotDot)},
		{name: "module command absolute", inv: module, cmd: cmd(workDir)},
		{name: "module implementation dot-dot", inv: module, impl: impl(dotDot)},
		{name: "module link", inv: module, cmd: cmd("link"), symlink: true},
	}
	// The CLI override naming the outside directory wins over a module
	// declaration of it at each level: trusted (the killer of the override
	// test in effectiveWorkDirSource).
	for _, level := range []harnessWorkdirKind{
		{name: "root", inv: withRoot(module, dotDot)},
		{name: "command", inv: module, cmd: cmd(workDir)},
		{name: "implementation", inv: module, impl: impl(workDir)},
	} {
		level.name = "cli override over module " + level.name
		level.override = invowkfile.WorkDir(workDir)
		level.trusted = true
		kinds = append(kinds, level)
	}
	return kinds
}

// TestVirtualPathHarness_RootConstruction replays the resolver construction
// (rows allowedRoots, anchors, access and workdirIsRoot) against the model's
// verdict: for every workdir kind, built the way a command run builds it
// (virtualWorkdirFor over an invowkfile, a command, an implementation and a
// CLI override), the real newVirtualPathResolverForFilesystem must accept a
// probe exactly when H.access = Full or locIsAllowedRoot[l, TrustedOrInside]
// (VirtualPathHarness.als), and EffectiveAccess must map an omitted access
// mode to restricted (openspec/specs/virtual-filesystem-access/spec.md,
// "Omitted virtual filesystem config is restricted").
//
// Probes are classified by the model's location kinds from how they are built,
// never from other virtual_policy.go helpers:
//   - InReadRoot: a file in the script base, the declared path when one is
//     declared, and each automatically allowed anchor the resolver reports
//     (@config, @data, @cache, @state, @tmp; openspec
//     cross-platform-path-anchors "Anchor allow roots are explicit"), and
//     @work when the workdir is trusted or inside the module;
//   - InWorkdir: a file under the workdir only (none when the workdir lies in
//     the script base: the model's ModuleInside, and the module default);
//   - OutsideAll: a path outside every root (and the undeclared path), a path
//     under @home that no other anchor encloses (the same requirement denies
//     blanket home access in restricted mode), and @work of a module-declared
//     workdir outside the module.
//
// The workdir, declared path, and outside probe are missing paths directly
// under the volume root, so no temp or anchor root encloses them; the link
// kind is a link inside the module to that workdir. The anchors row claims
// only that anchors are explicit allowed roots (decision 14 of
// harden-formal-instruments): the anchor paths are inputs from the production
// resolver, never compared with a per-OS table, which the model does not state.
func TestVirtualPathHarness_RootConstruction(t *testing.T) {
	t.Parallel()
	base := fstree.TempRoot(t)
	scriptBase := mustMkdir(t, filepath.Join(base, "module"))
	mustMkdir(t, filepath.Join(scriptBase, "sub"))
	scriptFile := filepath.Join(scriptBase, "script.sh")
	if err := os.WriteFile(scriptFile, []byte("echo"), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}
	volume := filepath.VolumeName(base) + string(filepath.Separator)
	tag := filepath.Base(base)
	workDir := filepath.Join(volume, "invowk-work-"+tag)
	declared := filepath.Join(volume, "invowk-declared-"+tag)
	outside := filepath.Join(volume, "invowk-outside-"+tag, "a", "b")

	for _, kind := range harnessWorkdirKinds(scriptBase, workDir) {
		t.Run(kind.name, func(t *testing.T) {
			t.Parallel()
			if kind.symlink {
				fstree.Require(t, fstree.CapSymlink)
				fstree.MustSymlink(t, workDir, filepath.Join(scriptBase, "link"))
			}
			workdir := virtualWorkdirFor(kind.inv, kind.cmd, kind.impl, kind.override)
			replayRootConstruction(t, workdir, kind, rootConstructionProbes{
				scriptBase: scriptBase, scriptFile: scriptFile, workDir: workDir,
				declared: declared, outside: outside, tag: tag,
			})
		})
	}
}

// replayRootConstruction builds the resolver of one workdir kind for every
// access mode, with and without a declared path, and compares its acceptance
// of each probe with the model.
func replayRootConstruction(t *testing.T, workdir virtualWorkdir, kind harnessWorkdirKind, p rootConstructionProbes) {
	t.Helper()
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
	// workdirIsRoot[TrustedOrInside]: trusted, or module-inside (enclosed).
	workdirRoot := kind.trusted || kind.enclosed
	for name, access := range accesses {
		for _, withDeclared := range []bool{false, true} {
			config := invowkfile.VirtualFilesystemConfig{Access: access}
			if withDeclared {
				config.Paths = invowkfile.VirtualFilesystemPaths{"DECLARED": invowkfile.VirtualFilesystemPath(p.declared)}
			}
			if got := config.EffectiveAccess(); got != wantEffective[name] {
				t.Errorf("access %s: EffectiveAccess = %q, want %q", name, got, wantEffective[name])
			}
			resolver, err := newVirtualPathResolverForFilesystem(workdir, p.scriptBase, config)
			if err != nil {
				t.Fatalf("access %s, declared %v: build resolver: %v", name, withDeclared, err)
			}

			probes := map[string]string{
				p.scriptFile:                            inReadRoot,
				p.outside:                               outsideAll,
				filepath.Join(p.declared, "x", "probe"): outsideAll,
			}
			if !kind.enclosed {
				probes[filepath.Join(p.workDir, "f")] = inWorkdir
			}
			if withDeclared {
				probes[filepath.Join(p.declared, "x", "probe")] = inReadRoot
			}
			for _, anchor := range []string{virtualAnchorConfig, virtualAnchorData, virtualAnchorCache, virtualAnchorState, virtualAnchorTmp, virtualAnchorWork} {
				root := resolver.anchors[anchor]
				if root == "" {
					t.Fatalf("access %s: the resolver reports no %s anchor", name, anchor)
				}
				probes[root] = inReadRoot
			}
			if !workdirRoot {
				probes[resolver.anchors[virtualAnchorWork]] = outsideAll
			}
			probes[homeProbe(t, resolver, p.tag)] = outsideAll

			full := wantEffective[name] == invowkfile.VirtualFilesystemAccessFull
			validator := virtualPathValidator{resolver: resolver}
			for probe, loc := range probes {
				// H.access = Full or locIsAllowedRoot[l, TrustedOrInside]
				want := full || loc == inReadRoot || (loc == inWorkdir && workdirRoot)
				_, err := validator.validate(p.scriptBase, probe)
				if got := err == nil; got != want {
					t.Errorf("access %s, declared %v: probe %s (%s) accepted=%v, model %v (err %v)",
						name, withDeclared, probe, loc, got, want, err)
				}
			}
		}
	}
}

// homeProbe returns the @home negative of cross-platform-path-anchors ("Home
// anchor does not grant implicit home access"): a missing path under
// @home/.ssh, which restricted access must deny. No allowed anchor may enclose
// it; on Windows %TEMP% and the AppData anchors sit under home but not under
// .ssh. It fails when the resolver reports no @home anchor.
func homeProbe(t *testing.T, resolver virtualPathResolver, tag string) string {
	t.Helper()
	home := resolver.anchors[virtualAnchorHome]
	if home == "" {
		t.Fatal("the resolver reports no @home anchor")
	}
	return filepath.Join(home, ".ssh", "invowk-home-probe-"+tag)
}
