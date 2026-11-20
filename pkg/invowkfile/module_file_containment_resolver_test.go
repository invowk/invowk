// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/internal/testutil/fstree"
)

// bothResolved holds the results of both resolvers on one script file, and
// every path the shared reader was asked for.
type bothResolved struct {
	implContent string
	implErr     error
	ccContent   ScriptContent
	ccErr       error
	calls       []string
}

// resolveBoth runs the implementation and the custom-check resolver on one
// script file, with a reader that records every path it is asked for.
func resolveBoth(t *testing.T, mod, file string) bothResolved {
	t.Helper()
	var r bothResolved
	reader := func(p string) ([]byte, error) {
		r.calls = append(r.calls, p)
		return os.ReadFile(p)
	}
	sfp := ScriptFilePath(file)
	impl := Implementation{Script: ImplementationScript{File: &sfp}}
	r.implContent, r.implErr = impl.ResolveScriptWithFSAndModule("", FilesystemPath(mod), reader)
	cc := CustomCheckScript{File: &sfp}
	r.ccContent, r.ccErr = cc.ResolveWithFSAndModule(FilesystemPath(mod), reader)
	return r
}

// TestResolvers_VendoredEscapingLinkDenied: a script.file that names an
// invowk_modules/ link to an outside file is denied by both resolvers with the
// traversal error, and the injected reader is never called (F13, SC-01).
func TestResolvers_VendoredEscapingLinkDenied(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	base, mod := containmentFixture(t)
	testutil.MustMkdirAll(t, filepath.Join(mod, "invowk_modules"), 0o755)
	fstree.MustSymlink(t, filepath.Join(base, "outside"), filepath.Join(mod, "invowk_modules", "x"))

	r := resolveBoth(t, mod, "invowk_modules/x")

	implContent := r.implContent

	implErr := r.implErr

	ccContent := r.ccContent

	ccErr := r.ccErr

	calls := r.calls
	if !errors.Is(implErr, ErrScriptPathTraversal) || implContent != "" {
		t.Errorf("implementation: content %q err %v, want the traversal error", implContent, implErr)
	}
	if !errors.Is(ccErr, ErrScriptPathTraversal) || ccContent != "" {
		t.Errorf("custom check: content %q err %v, want the traversal error", ccContent, ccErr)
	}
	if len(calls) != 0 {
		t.Errorf("reader called for %v, want no call", calls)
	}
}

// TestResolvers_ContainedAndMissingStillReach: a regular file, a real vendored
// file and a relative in-module link are read; a missing file still reaches the
// reader, which reports the read error, not the traversal error.
func TestResolvers_ContainedAndMissingStillReach(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	_, mod := containmentFixture(t)
	testutil.MustMkdirAll(t, filepath.Join(mod, "invowk_modules", "dep.invowkmod"), 0o755)
	mustWrite(t, filepath.Join(mod, "invowk_modules", "dep.invowkmod", "run.sh"), "echo vendored")
	fstree.MustSymlink(t, filepath.Join("..", "script.sh"), filepath.Join(mod, "invowk_modules", "rel"))

	for file, want := range map[string]string{
		"script.sh":                           "echo inside",
		"invowk_modules/dep.invowkmod/run.sh": "echo vendored",
		"invowk_modules/rel":                  "echo inside",
	} {
		r := resolveBoth(t, mod, file)
		implContent := r.implContent
		implErr := r.implErr
		ccContent := r.ccContent
		ccErr := r.ccErr
		calls := r.calls
		if implErr != nil || implContent != want || ccErr != nil || string(ccContent) != want {
			t.Errorf("%s: impl %q %v, custom check %q %v, want %q", file, implContent, implErr, ccContent, ccErr, want)
		}
		if len(calls) != 2 {
			t.Errorf("%s: reader calls %v, want one per resolver", file, calls)
		}
	}

	r := resolveBoth(t, mod, "missing.sh")

	implErr := r.implErr

	ccErr := r.ccErr

	calls := r.calls
	for name, err := range map[string]error{"implementation": implErr, "custom check": ccErr} {
		if err == nil || errors.Is(err, ErrScriptPathTraversal) || !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s missing file: err %v, want the not-exist read error", name, err)
		}
	}
	if len(calls) != 2 {
		t.Errorf("missing file: reader calls %v, want one per resolver", calls)
	}
}

// TestResolvers_DanglingAndCycleDenied: unresolvable links are denied before
// the reader runs, naming the case rather than an escape.
func TestResolvers_DanglingAndCycleDenied(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	_, mod := containmentFixture(t)
	fstree.MustSymlink(t, "missing", filepath.Join(mod, "dangling"))
	fstree.MustSymlink(t, "cycle", filepath.Join(mod, "cycle"))
	for file, msg := range map[string]string{"dangling": "dangling link", "cycle": "link does not resolve"} {
		r := resolveBoth(t, mod, file)
		implErr := r.implErr
		ccErr := r.ccErr
		calls := r.calls
		for name, err := range map[string]error{"implementation": implErr, "custom check": ccErr} {
			if !errors.Is(err, ErrScriptPathTraversal) || !strings.Contains(err.Error(), msg) {
				t.Errorf("%s %s: err %v, want a traversal error naming %q", file, name, err, msg)
			}
		}
		if len(calls) != 0 {
			t.Errorf("%s: reader called for %v", file, calls)
		}
	}
}

// TestResolvers_PhysicalContainmentAliasAndCase: containment is judged
// physically, so a module path given through an aliased parent, or (on a
// case-insensitive filesystem) in a different case, still reads a contained
// script (decision 46 item 1).
//
//   - alias: on macOS the unresolved os.TempDir() under /var (-> /private/var);
//     elsewhere a symlinked parent directory (skipped only where symlinks are
//     neither available nor required, per the fstree capability policy);
//   - case: runs on case-insensitive filesystems (macOS and Windows by
//     default) and is skipped, with that reason, on case-sensitive ones.
func TestResolvers_PhysicalContainmentAliasAndCase(t *testing.T) {
	t.Parallel()

	t.Run("alias", func(t *testing.T) {
		t.Parallel()
		mod := aliasedModuleDir(t)
		mustWrite(t, filepath.Join(mod, "script.sh"), "echo alias")
		r := resolveBoth(t, mod, "script.sh")
		implContent := r.implContent
		implErr := r.implErr
		ccContent := r.ccContent
		ccErr := r.ccErr
		if implErr != nil || implContent != "echo alias" || ccErr != nil || string(ccContent) != "echo alias" {
			t.Fatalf("alias %s: impl %q %v, custom check %q %v", mod, implContent, implErr, ccContent, ccErr)
		}
	})

	t.Run("case", func(t *testing.T) {
		t.Parallel()
		base := fstree.TempRoot(t)
		if !caseInsensitiveDir(t, base) {
			t.Skipf("filesystem at %s is case-sensitive on %s; the case variant needs a case-insensitive one", base, runtime.GOOS)
		}
		mod := filepath.Join(base, "CaseMod")
		testutil.MustMkdirAll(t, mod, 0o755)
		mustWrite(t, filepath.Join(mod, "Script.sh"), "echo case")
		r := resolveBoth(t, filepath.Join(base, "casemod"), "script.SH")
		implContent := r.implContent
		implErr := r.implErr
		ccContent := r.ccContent
		ccErr := r.ccErr
		if implErr != nil || implContent != "echo case" || ccErr != nil || string(ccContent) != "echo case" {
			t.Fatalf("case: impl %q %v, custom check %q %v", implContent, implErr, ccContent, ccErr)
		}
	})
}

// aliasedModuleDir returns a module directory whose given path differs from
// its physical path.
func aliasedModuleDir(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "darwin" {
		raw := t.TempDir()
		if resolved, err := filepath.EvalSymlinks(raw); err != nil || resolved == raw {
			t.Fatalf("macOS temp dir %s is not aliased (resolved %q, %v)", raw, resolved, err)
		}
		mod := filepath.Join(raw, "mod")
		testutil.MustMkdirAll(t, mod, 0o755)
		return mod
	}
	fstree.Require(t, fstree.CapSymlink)
	base := fstree.TempRoot(t)
	realDir := filepath.Join(base, "real")
	testutil.MustMkdirAll(t, filepath.Join(realDir, "mod"), 0o755)
	fstree.MustSymlink(t, realDir, filepath.Join(base, "alias"))
	return filepath.Join(base, "alias", "mod")
}

// caseInsensitiveDir reports whether names in dir are case-insensitive.
func caseInsensitiveDir(t *testing.T, dir string) bool {
	t.Helper()
	mustWrite(t, filepath.Join(dir, "CaseProbe"), "")
	_, err := os.Stat(filepath.Join(dir, "caseprobe"))
	return err == nil
}
