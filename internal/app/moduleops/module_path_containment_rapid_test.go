// SPDX-License-Identifier: MPL-2.0

package moduleops

import (
	"os"
	"path/filepath"
	"testing"

	"pgregory.net/rapid"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

// TestModulePathContainment_Property generates module trees with in-module,
// vendored, and escaping links at larger scope than the golden, materialises
// them, and checks the composed real behaviour: a script read that a module can
// trigger, judged by the resolver's result, stays physically inside the
// module. Every escape is a policy violation: an invowk_modules/ link of
// either kind (the fixed F13 hole) and a non-vendored junction (the fixed F16
// hole) included. Symlink targets are drawn absolute or relative to the
// link's directory; junction targets are always absolute.
//
// Links are symlinks, or directory junctions where the host offers them
// (required on Windows, design D6 of harden-formal-instruments). A junction
// targets a directory, so its script path is <rel>x/secret. Escape is judged by
// file identity against a unique outside sentinel, backed by its content, never
// by filepath.EvalSymlinks: since Go 1.23 a junction is not ModeSymlink and
// EvalSymlinks may not evaluate it.
func TestModulePathContainment_Property(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	if fstree.CapJunction.Required(fstree.CurrentHost()) {
		fstree.Require(t, fstree.CapJunction)
	}
	junctions := fstree.Probe(t).Junction
	root := fstree.TempRoot(t)

	rapid.Check(t, func(rt *rapid.T) {
		dir, err := os.MkdirTemp(root, "case-*")
		if err != nil {
			rt.Fatalf("mkdtemp: %v", err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		mod := filepath.Join(dir, "m0.invowkmod")
		if err = os.MkdirAll(mod, 0o755); err != nil {
			rt.Fatalf("mkdir mod: %v", err)
		}
		fstree.WriteModuleFiles(rt, mod)
		// an ordinary in-module script file, always safe
		if err = os.WriteFile(filepath.Join(mod, "script.sh"), []byte("echo hi"), 0o644); err != nil {
			rt.Fatalf("write script: %v", err)
		}
		// the unique outside sentinel every escape is judged against
		outsideDir := filepath.Join(dir, "outside")
		sentinel := filepath.Join(outsideDir, "secret")
		marker := "OUTSIDE-" + filepath.Base(dir)
		if err = os.MkdirAll(outsideDir, 0o755); err != nil {
			rt.Fatalf("mkdir outside: %v", err)
		}
		if err = os.WriteFile(sentinel, []byte(marker), 0o644); err != nil {
			rt.Fatalf("write sentinel: %v", err)
		}

		scriptPath := "script.sh"
		junction := junctions && rapid.Bool().Draw(rt, "junction")
		vendored := rapid.Bool().Draw(rt, "vendored")
		placeLink := rapid.Bool().Draw(rt, "placeLink")
		escaping := rapid.Bool().Draw(rt, "escaping")
		if placeLink {
			linkDir := mod
			rel := ""
			if vendored {
				linkDir = filepath.Join(mod, "invowk_modules")
				rel = "invowk_modules/"
				if err = os.MkdirAll(linkDir, 0o755); err != nil {
					rt.Fatalf("mkdir vendor: %v", err)
				}
			}
			linkPath := rel + "x"
			if junction {
				// A junction targets a directory: an in-module one, or the outside one.
				target := outsideDir
				if !escaping {
					target = filepath.Join(mod, "inner")
					if err = os.MkdirAll(target, 0o755); err != nil {
						rt.Fatalf("mkdir inner: %v", err)
					}
					if err = os.WriteFile(filepath.Join(target, "secret"), []byte("inside"), 0o644); err != nil {
						rt.Fatalf("write inner: %v", err)
					}
				}
				fstree.MustJunction(rt, target, filepath.Join(linkDir, "x"))
				linkPath += "/secret"
			} else {
				target := filepath.Join(mod, "script.sh")
				if escaping {
					target = sentinel
				}
				if rapid.Bool().Draw(rt, "relativeTarget") {
					if target, err = filepath.Rel(linkDir, target); err != nil {
						rt.Fatalf("relative target: %v", err)
					}
				}
				fstree.MustSymlink(rt, target, filepath.Join(linkDir, "x"))
			}
			if rapid.Bool().Draw(rt, "useLink") {
				scriptPath = linkPath
			}
		}

		// Only an admitted module can trigger reads (discovery gates them).
		if _, loadErr := invowkmod.Load(types.FilesystemPath(mod)); loadErr != nil {
			return
		}

		sfp := invowkfile.ScriptFilePath(scriptPath)
		impl := invowkfile.Implementation{Script: invowkfile.ImplementationScript{File: &sfp}}
		opened := ""
		reader := func(p string) ([]byte, error) {
			opened = p
			return os.ReadFile(p)
		}
		content, err := impl.ResolveScriptWithFSAndModule("", invowkfile.FilesystemPath(mod), reader)
		if err != nil {
			return // denied (or a read error): nothing was accepted
		}
		if opened == "" {
			rt.Fatalf("script %q accepted without a read", scriptPath)
		}
		escaped := sameFile(rt, opened, sentinel)
		if markerRead := content == marker; markerRead != escaped {
			rt.Fatalf("escape oracle disagrees for %q: same file as the sentinel %v, sentinel content read %v", opened, escaped, markerRead)
		}
		if !escaped {
			return // contained: the expected case
		}
		// Every escape fails, through invowk_modules/ (F13, fixed) or through
		// a non-vendored junction that Load's scan admits (F16, fixed by the
		// physical read check: windows-latest CI run 44648263846).
		rt.Fatalf("policy violation: script %q (junction %v) escaped the module to the outside sentinel", scriptPath, junction)
	})
}

// sameFile reports whether path is the same file as sentinel (os.SameFile).
func sameFile(rt *rapid.T, path, sentinel string) bool {
	opened, err := os.Stat(path)
	if err != nil {
		return false
	}
	want, err := os.Stat(sentinel)
	if err != nil {
		rt.Fatalf("stat sentinel: %v", err)
	}
	return os.SameFile(opened, want)
}
