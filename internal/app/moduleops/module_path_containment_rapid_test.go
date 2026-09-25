// SPDX-License-Identifier: MPL-2.0

package moduleops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

// TestModulePathContainment_Property generates module trees with in-module,
// vendored, and escaping symlinks at larger scope than the golden, materialises
// them, and checks the composed real behaviour: a script read that a module can
// trigger stays physically inside the module, unless it is the recorded F13
// invowk_modules/ hole. Any other escape is a policy violation.
func TestModulePathContainment_Property(t *testing.T) {
	t.Parallel()
	if !fstree.Probe(t).Symlink {
		t.Skip("symlink creation unavailable on this host")
	}
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

		scriptPath := "script.sh"
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
			var target string
			if escaping {
				target = filepath.Join(dir, "secret")
				if err = os.WriteFile(target, []byte("OUTSIDE"), 0o644); err != nil {
					rt.Fatalf("write secret: %v", err)
				}
			} else {
				target = filepath.Join(mod, "script.sh")
			}
			if err = os.Symlink(target, filepath.Join(linkDir, "x")); err != nil {
				rt.Skipf("symlink: %v", err)
			}
			if rapid.Bool().Draw(rt, "useLink") {
				scriptPath = rel + "x"
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
		if _, err = impl.ResolveScriptWithFSAndModule("", invowkfile.FilesystemPath(mod), reader); err != nil || opened == "" {
			return
		}
		resolved, err := filepath.EvalSymlinks(opened)
		if err != nil {
			return
		}
		if fstree.Within(mod, resolved) {
			return // contained: the expected case
		}
		// An escape is only acceptable as the recorded F13 finding: the path
		// crossed an invowk_modules/ symlink.
		if !strings.HasPrefix(filepath.ToSlash(scriptPath), "invowk_modules/") {
			rt.Fatalf("policy violation: script %q escaped the module to %q without the invowk_modules hole", scriptPath, resolved)
		}
	})
}
