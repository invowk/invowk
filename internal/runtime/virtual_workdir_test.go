// SPDX-License-Identifier: MPL-2.0

package runtime

import (
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
	// workdirTree is the tree the workdir-root unit tests build: a module
	// directory with a subdirectory, and a sibling directory outside the module.
	workdirTree struct {
		base   string
		module string
		inside string
		shared string
	}

	// workdirCase is one source of the effective workdir and the expected decision.
	workdirCase struct {
		name     string
		inv      *invowkfile.Invowkfile
		cmd      *invowkfile.Command
		impl     *invowkfile.Implementation
		override invowkfile.WorkDir
		// declared is whether virtualWorkdirFor must record the module root.
		declared bool
		root     bool
		// wantPath is the expected effective workdir, when the case checks it.
		wantPath string
	}
)

func newWorkdirTree(t *testing.T) workdirTree {
	t.Helper()
	base := fstree.TempRoot(t)
	module := mustMkdir(t, filepath.Join(base, "mod.invowkmod"))
	return workdirTree{
		base:   base,
		module: module,
		inside: mustMkdir(t, filepath.Join(module, "sub")),
		shared: mustMkdir(t, filepath.Join(base, "shared")),
	}
}

// moduleInvowkfile is the invowkfile of the tree's module.
func (tree workdirTree) moduleInvowkfile() *invowkfile.Invowkfile {
	return &invowkfile.Invowkfile{
		FilePath:   invowkfile.FilesystemPath(filepath.Join(tree.module, "invowkfile.cue")),
		ModulePath: invowkfile.FilesystemPath(tree.module),
	}
}

func workdirCommand(workdir invowkfile.WorkDir) *invowkfile.Command {
	return &invowkfile.Command{Name: "test", WorkDir: workdir}
}

func workdirImplementation(workdir invowkfile.WorkDir) *invowkfile.Implementation {
	return &invowkfile.Implementation{Script: invowkfile.ImplementationScript{Content: "echo"}, WorkDir: workdir}
}

// TestVirtualWorkdir covers every source of the effective workdir: whether
// virtualWorkdirFor records the module root, what isReadRoot decides, and
// whether the workdir and @work are in the resolver's normalised allowed roots
// (virtual-runtime-sandbox, "The work directory is an allowed root unless a
// module declares it outside the module").
func TestVirtualWorkdir(t *testing.T) {
	t.Parallel()
	tree := newWorkdirTree(t)

	plainDir := mustMkdir(t, filepath.Join(tree.base, "plain"))
	plain := &invowkfile.Invowkfile{FilePath: invowkfile.FilesystemPath(filepath.Join(plainDir, "invowkfile.cue"))}
	rootLevel := tree.moduleInvowkfile()
	rootLevel.WorkDir = "../shared"

	cases := []workdirCase{
		{
			name: "cli override outside the module", inv: tree.moduleInvowkfile(), cmd: workdirCommand("../shared"),
			override: invowkfile.WorkDir(tree.shared), root: true, wantPath: tree.shared,
		},
		{
			name: "plain invowkfile workdir outside its directory", inv: plain, cmd: workdirCommand("../shared"),
			root: true, wantPath: tree.shared,
		},
		{
			name: "module default", inv: tree.moduleInvowkfile(), cmd: workdirCommand(""),
			root: true, wantPath: tree.module,
		},
		{
			name: "module inside", inv: tree.moduleInvowkfile(), cmd: workdirCommand("sub"),
			declared: true, root: true, wantPath: tree.inside,
		},
		{
			name: "module missing workdir below the module", inv: tree.moduleInvowkfile(), cmd: workdirCommand("missing/deeper"),
			declared: true, root: true,
		},
		{
			name: "module outside by dot-dot at command level", inv: tree.moduleInvowkfile(), cmd: workdirCommand("../shared"),
			declared: true, root: false, wantPath: tree.shared,
		},
		{
			name: "module outside by dot-dot at root level", inv: rootLevel, cmd: workdirCommand(""),
			declared: true, root: false, wantPath: tree.shared,
		},
		{
			name: "module outside by absolute path at implementation level", inv: tree.moduleInvowkfile(),
			cmd: workdirCommand("sub"), impl: workdirImplementation(invowkfile.WorkDir(tree.shared)),
			declared: true, root: false, wantPath: tree.shared,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			workdir := virtualWorkdirFor(tc.inv, tc.cmd, tc.impl, tc.override)
			if tc.wantPath != "" && workdir.path != tc.wantPath {
				t.Fatalf("workdir path = %q, want %q", workdir.path, tc.wantPath)
			}
			if (workdir.moduleRoot != "") != tc.declared {
				t.Fatalf("moduleRoot = %q, want set=%v", workdir.moduleRoot, tc.declared)
			}
			expectWorkdirRoot(t, workdir, tc.inv, tc.root)
			if tc.root {
				expectWorkdirReadAccepted(t, workdir, tc.inv, invowkfile.VirtualFilesystemAccessRestricted)
			}
		})
	}

	t.Run("through a link to an outside directory", func(t *testing.T) {
		t.Parallel()
		fstree.Require(t, fstree.CapSymlink)
		tree := newWorkdirTree(t)
		fstree.MustSymlink(t, tree.shared, filepath.Join(tree.module, "link"))
		inv := tree.moduleInvowkfile()
		workdir := virtualWorkdirFor(inv, workdirCommand("link"), nil, "")
		expectWorkdirRoot(t, workdir, inv, false)
	})

	t.Run("through a link that stays in the module", func(t *testing.T) {
		t.Parallel()
		fstree.Require(t, fstree.CapSymlink)
		tree := newWorkdirTree(t)
		fstree.MustSymlink(t, tree.inside, filepath.Join(tree.module, "inlink"))
		inv := tree.moduleInvowkfile()
		workdir := virtualWorkdirFor(inv, workdirCommand("inlink"), nil, "")
		expectWorkdirRoot(t, workdir, inv, true)
		expectWorkdirReadAccepted(t, workdir, inv, invowkfile.VirtualFilesystemAccessRestricted)
	})

	t.Run("link cycle", func(t *testing.T) {
		t.Parallel()
		fstree.Require(t, fstree.CapSymlink)
		tree := newWorkdirTree(t)
		fstree.MustSymlink(t, "cycle", filepath.Join(tree.module, "cycle"))
		inv := tree.moduleInvowkfile()
		workdir := virtualWorkdirFor(inv, workdirCommand("cycle/x"), nil, "")
		if workdir.isReadRoot() {
			t.Fatal("isReadRoot() = true for a workdir that cannot be resolved")
		}
		resolver := mustWorkdirResolver(t, workdir, inv, invowkfile.VirtualFilesystemAccessRestricted)
		cycle := filepath.Join(tree.module, "cycle")
		if slices.ContainsFunc(resolver.allowedRoots, func(root string) bool { return fstree.Within(cycle, root) }) {
			t.Fatalf("the unresolvable workdir is an allowed root: %v", resolver.allowedRoots)
		}
	})

	t.Run("outside the module at the volume root", func(t *testing.T) {
		t.Parallel()
		tree := newWorkdirTree(t)
		inv := tree.moduleInvowkfile()
		// A missing directory under the volume root: no temp or anchor root
		// encloses it, so the resolver's verdict shows the decision.
		volume := filepath.VolumeName(tree.base) + string(filepath.Separator)
		outside := filepath.Join(volume, "invowk-workdir-outside-"+filepath.Base(tree.base), "w")
		workdir := virtualWorkdirFor(inv, nil, workdirImplementation(invowkfile.WorkDir(outside)), "")
		expectWorkdirRoot(t, workdir, inv, false)
		resolver := mustWorkdirResolver(t, workdir, inv, invowkfile.VirtualFilesystemAccessRestricted)
		validator := virtualPathValidator{resolver: resolver}
		if _, err := validator.validate(tree.module, filepath.Join(outside, "data.txt")); err == nil {
			t.Fatal("a read below a module-declared workdir outside the module was accepted")
		} else if !strings.Contains(err.Error(), "virtual path denied") {
			t.Fatalf("denial = %v, want virtual path denied", err)
		}
		expectWorkdirReadAccepted(t, workdir, inv, invowkfile.VirtualFilesystemAccessFull)

		trusted := virtualWorkdirFor(inv, nil, nil, invowkfile.WorkDir(outside))
		expectWorkdirRoot(t, trusted, inv, true)
		expectWorkdirReadAccepted(t, trusted, inv, invowkfile.VirtualFilesystemAccessRestricted)
	})

	if runtime.GOOS == "darwin" {
		t.Run("darwin private aliases", func(t *testing.T) {
			t.Parallel()
			testVirtualWorkdirDarwinAliases(t)
		})
	}
	if runtime.GOOS == goosWindows {
		t.Run("windows missing drive", func(t *testing.T) {
			t.Parallel()
			testVirtualWorkdirWindowsMissingDrive(t)
		})
		t.Run("windows other volume", func(t *testing.T) {
			t.Parallel()
			testVirtualWorkdirWindowsOtherVolume(t)
		})
		t.Run("windows junction to an outside directory", func(t *testing.T) {
			t.Parallel()
			fstree.Require(t, fstree.CapJunction)
			tree := newWorkdirTree(t)
			fstree.MustJunction(t, tree.shared, filepath.Join(tree.module, "j"))
			inv := tree.moduleInvowkfile()
			workdir := virtualWorkdirFor(inv, workdirCommand("j"), nil, "")
			expectWorkdirRoot(t, workdir, inv, false)
		})
	}
}

// testVirtualWorkdirDarwinAliases writes the module through /var and the
// workdir through /private/var, and the other way round: the decision is the
// one for the resolved paths.
func testVirtualWorkdirDarwinAliases(t *testing.T) {
	t.Helper()
	tree := newWorkdirTree(t)
	if !strings.HasPrefix(tree.module, "/private/") {
		t.Fatalf("temp root %q is not below /private on darwin", tree.module)
	}
	alias := strings.TrimPrefix(tree.module, "/private")
	for _, tc := range []struct {
		name, module, inside, outside string
	}{
		{"module through /var", alias, filepath.Join(tree.module, "sub"), tree.shared},
		{"workdir through /var", tree.module, filepath.Join(alias, "sub"), strings.TrimPrefix(tree.shared, "/private")},
	} {
		inside := virtualWorkdir{path: tc.inside, moduleRoot: tc.module}
		if !inside.isReadRoot() {
			t.Errorf("%s: isReadRoot() = false for a workdir inside the module", tc.name)
		}
		outside := virtualWorkdir{path: tc.outside, moduleRoot: tc.module}
		if outside.isReadRoot() {
			t.Errorf("%s: isReadRoot() = true for a workdir outside the module", tc.name)
		}
	}
}

// testVirtualWorkdirWindowsMissingDrive covers the resolution-error branch: a
// workdir on a drive letter that does not exist counts as outside.
func testVirtualWorkdirWindowsMissingDrive(t *testing.T) {
	t.Helper()
	tree := newWorkdirTree(t)
	missing := ""
	for letter := 'Z'; letter >= 'D'; letter-- {
		drive := string(letter) + `:\`
		if _, err := os.Stat(drive); err != nil {
			missing = drive
			break
		}
	}
	if missing == "" {
		t.Skip("every drive letter from D to Z exists on this host")
	}
	inv := tree.moduleInvowkfile()
	workdir := virtualWorkdirFor(inv, nil, workdirImplementation(invowkfile.WorkDir(missing+"work")), "")
	if workdir.isReadRoot() {
		t.Fatalf("isReadRoot() = true for a workdir on the missing drive %s", missing)
	}
}

// testVirtualWorkdirWindowsOtherVolume covers the cross-volume branch of
// pathWithin: a workdir on another existing volume counts as outside.
func testVirtualWorkdirWindowsOtherVolume(t *testing.T) {
	t.Helper()
	tree := newWorkdirTree(t)
	volume := filepath.VolumeName(tree.module)
	other := ""
	candidates := []string{}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.VolumeName(wd))
	}
	for letter := 'C'; letter <= 'Z'; letter++ {
		candidates = append(candidates, string(letter)+":")
	}
	for _, candidate := range candidates {
		if candidate == "" || strings.EqualFold(candidate, volume) {
			continue
		}
		if _, err := os.Stat(candidate + `\`); err == nil {
			other = candidate
			break
		}
	}
	if other == "" {
		t.Skipf("no existing volume other than %s on this host", volume)
	}
	inv := tree.moduleInvowkfile()
	workdir := virtualWorkdirFor(inv, nil, workdirImplementation(invowkfile.WorkDir(other+`\invowk-workdir-other-volume`)), "")
	expectWorkdirRoot(t, workdir, inv, false)
}

// mustWorkdirResolver builds the resolver of a command run with the workdir.
func mustWorkdirResolver(
	t *testing.T,
	workdir virtualWorkdir,
	inv *invowkfile.Invowkfile,
	access invowkfile.VirtualFilesystemAccess,
) virtualPathResolver {
	t.Helper()
	resolver, err := newVirtualPathResolverForFilesystem(workdir, string(inv.GetScriptBasePath()),
		invowkfile.VirtualFilesystemConfig{Access: access})
	if err != nil {
		t.Fatalf("build resolver: %v", err)
	}
	return resolver
}

// expectWorkdirRoot checks isReadRoot, and whether the workdir and the @work
// anchor are in the resolver's normalised allowed roots. @work reports the
// workdir either way.
func expectWorkdirRoot(t *testing.T, workdir virtualWorkdir, inv *invowkfile.Invowkfile, want bool) {
	t.Helper()
	if got := workdir.isReadRoot(); got != want {
		t.Fatalf("isReadRoot() = %v, want %v (workdir %q, module root %q)", got, want, workdir.path, workdir.moduleRoot)
	}
	resolver := mustWorkdirResolver(t, workdir, inv, invowkfile.VirtualFilesystemAccessRestricted)
	if got := resolver.anchors[virtualAnchorWork]; got != workdir.path {
		t.Fatalf("@work = %q, want the workdir %q", got, workdir.path)
	}
	normalized, err := normalizeExistingOrParent(workdir.path, "")
	if err != nil {
		t.Fatalf("normalize workdir %q: %v", workdir.path, err)
	}
	if got := slices.Contains(resolver.allowedRoots, normalized); got != want {
		t.Fatalf("workdir %q in allowed roots = %v, want %v: %v", normalized, got, want, resolver.allowedRoots)
	}
}

// expectWorkdirReadAccepted checks that a read below the workdir is accepted.
func expectWorkdirReadAccepted(
	t *testing.T,
	workdir virtualWorkdir,
	inv *invowkfile.Invowkfile,
	access invowkfile.VirtualFilesystemAccess,
) {
	t.Helper()
	resolver := mustWorkdirResolver(t, workdir, inv, access)
	validator := virtualPathValidator{resolver: resolver}
	if _, err := validator.validate(workdir.path, filepath.Join(workdir.path, "data.txt")); err != nil {
		t.Fatalf("read below the workdir %q denied under %s access: %v", workdir.path, access.String(), err)
	}
}
