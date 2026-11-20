// SPDX-License-Identifier: MPL-2.0

package moduleops

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/app/modulecache"
	"github.com/invowk/invowk/internal/provision"
	ivkruntime "github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

// vendoredSymlinkModule builds a module admitted by Load whose invowk_modules/x
// entry is a symlink to a file outside the module, and returns the module root
// and the secret file's content.
func vendoredSymlinkModule(t *testing.T) (moduleRoot, secret string) {
	t.Helper()
	fstree.Require(t, fstree.CapSymlink)
	base := fstree.TempRoot(t)
	mod := filepath.Join(base, "m0.invowkmod")
	if err := os.MkdirAll(filepath.Join(mod, "invowk_modules"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	fstree.WriteModuleFiles(t, mod)
	secretPath := filepath.Join(base, "secret")
	secret = "OUTSIDE-SECRET"
	if err := os.WriteFile(secretPath, []byte(secret), 0o644); err != nil {
		t.Fatalf("write secret: %v", err)
	}
	fstree.MustSymlink(t, secretPath, filepath.Join(mod, "invowk_modules", "x"))
	// The module is admitted: Load skips invowk_modules by exact name.
	if !invowkmod.IsModule(types.FilesystemPath(mod)) {
		t.Fatalf("IsModule rejected the module unexpectedly")
	}
	if _, err := invowkmod.Load(types.FilesystemPath(mod)); err != nil {
		t.Fatalf("Load rejected the module unexpectedly: %v", err)
	}
	return mod, secret
}

// TestModulePathContainment_VendoredSymlinkScriptReadDenied is the regression
// replay of fixed finding F13 (pre-fix mutant
// mutantPreFixF13LexicalScriptContainment): a module's script.file into
// invowk_modules/ reaches a symlink Load never scans, and the physical
// containment check denies the read, for the implementation script and for a
// custom-check script, without returning the outside secret or calling the
// reader.
func TestModulePathContainment_VendoredSymlinkScriptReadDenied(t *testing.T) {
	t.Parallel()
	mod, secret := vendoredSymlinkModule(t)

	calls := 0
	reader := func(p string) ([]byte, error) {
		calls++
		return os.ReadFile(p)
	}
	sfp := invowkfile.ScriptFilePath("invowk_modules/x")
	impl := invowkfile.Implementation{Script: invowkfile.ImplementationScript{File: &sfp}}
	content, err := impl.ResolveScriptWithFSAndModule("", invowkfile.FilesystemPath(mod), reader)
	if !errors.Is(err, invowkfile.ErrScriptPathTraversal) {
		t.Fatalf("script read through the vendored link: err = %v, want ErrScriptPathTraversal", err)
	}
	if strings.Contains(content, secret) {
		t.Fatalf("script read returned the outside secret %q", content)
	}

	cc := invowkfile.CustomCheckScript{File: &sfp}
	ccContent, ccErr := cc.ResolveWithFSAndModule(invowkfile.FilesystemPath(mod), reader)
	if !errors.Is(ccErr, invowkfile.ErrScriptPathTraversal) {
		t.Fatalf("custom-check read through the vendored link: err = %v, want ErrScriptPathTraversal", ccErr)
	}
	if strings.Contains(string(ccContent), secret) {
		t.Fatalf("custom-check read returned the outside secret %q", ccContent)
	}
	if calls != 0 {
		t.Fatalf("the injected reader was called %d times, want none", calls)
	}
}

// TestModulePathContainment_AbsoluteInModuleLinkDenied pins the one case in
// which the code is stricter than the model (readOp and physContains rows): a
// link with an absolute target is denied even when the target lies inside the
// module, because os.Root rejects absolute link targets. It covers a target in
// a subdirectory and a module path given through an aliased parent, outside
// the golden scope, for both script resolvers and for the module env read;
// the same link with a relative target is followed.
func TestModulePathContainment_AbsoluteInModuleLinkDenied(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)
	base := fstree.TempRoot(t)
	realMod := filepath.Join(base, "real", "m0.invowkmod")
	if err := os.MkdirAll(filepath.Join(realMod, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	fstree.WriteModuleFiles(t, realMod)
	if err := os.WriteFile(filepath.Join(realMod, "sub", "target"), []byte("INSIDE=1\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	fstree.MustSymlink(t, filepath.Join(realMod, "sub", "target"), filepath.Join(realMod, "abs"))
	fstree.MustSymlink(t, filepath.Join("sub", "target"), filepath.Join(realMod, "rel"))
	fstree.MustSymlink(t, filepath.Join(base, "real"), filepath.Join(base, "alias"))

	reads := map[string]func(mod, file string) (string, error){
		"Implementation.ResolveScriptWithFSAndModule": scriptRead,
		"CustomCheckScript.ResolveWithFSAndModule":    customCheckRead,
	}
	for _, mod := range []string{realMod, filepath.Join(base, "alias", "m0.invowkmod")} {
		env := map[string]string{}
		if err := ivkruntime.LoadEnvFile(env, "abs", mod, mod); !errors.Is(err, ivkruntime.ErrEnvFileOutsideModule) || len(env) != 0 {
			t.Errorf("LoadEnvFile, module %s: read through an absolute in-module link: env %v, err = %v; want ErrEnvFileOutsideModule", mod, env, err)
		}
		if err := ivkruntime.LoadEnvFile(env, "rel", mod, mod); err != nil || env["INSIDE"] != "1" {
			t.Errorf("LoadEnvFile, module %s: read through a relative in-module link: env %v, err = %v; want INSIDE=1", mod, env, err)
		}
		for name, read := range reads {
			if _, err := read(mod, "abs"); !errors.Is(err, invowkfile.ErrScriptPathTraversal) {
				t.Errorf("%s, module %s: read through an absolute in-module link: err = %v, want ErrScriptPathTraversal", name, mod, err)
			}
			if content, err := read(mod, "rel"); err != nil || content != "INSIDE=1\n" {
				t.Errorf("%s, module %s: read through a relative in-module link = %q, %v; want the target", name, mod, content, err)
			}
		}
	}
}

// scriptRead resolves an implementation script file of the module at mod.
func scriptRead(mod, file string) (string, error) {
	sfp := invowkfile.ScriptFilePath(file)
	impl := invowkfile.Implementation{Script: invowkfile.ImplementationScript{File: &sfp}}
	return impl.ResolveScriptWithFSAndModule("", invowkfile.FilesystemPath(mod), os.ReadFile)
}

// customCheckRead resolves a custom-check script file of the module at mod.
func customCheckRead(mod, file string) (string, error) {
	sfp := invowkfile.ScriptFilePath(file)
	cc := invowkfile.CustomCheckScript{File: &sfp}
	content, err := cc.ResolveWithFSAndModule(invowkfile.FilesystemPath(mod), os.ReadFile)
	return string(content), err
}

// TestModulePathContainment_VendoredSymlinkEnvReadDenied is the regression
// replay of fixed finding F17 (pre-fix mutant
// mutantPreFixF17EnvNoPhysicalContainment): an env file declared as
// invowk_modules/x reaches the same symlink hole, and the module-context read
// fails with ErrEnvFileOutsideModule, the optional form included, without
// loading the outside variable.
func TestModulePathContainment_VendoredSymlinkEnvReadDenied(t *testing.T) {
	t.Parallel()
	mod, _ := vendoredSymlinkModule(t)
	// point the symlink at a real dotenv file
	secretEnv := filepath.Join(filepath.Dir(mod), "secret")
	if err := os.WriteFile(secretEnv, []byte("LEAKED=1\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	if err := invowkfile.ValidateEnvFilePath("invowk_modules/x"); err != nil {
		t.Fatalf("precondition: ValidateEnvFilePath rejected the path: %v", err)
	}
	for _, path := range []string{"invowk_modules/x", "invowk_modules/x?"} {
		env := map[string]string{}
		err := ivkruntime.LoadEnvFile(env, path, mod, mod)
		if !errors.Is(err, ivkruntime.ErrEnvFileOutsideModule) {
			t.Errorf("%s: err = %v, want ErrEnvFileOutsideModule", path, err)
		}
		if len(env) != 0 {
			t.Errorf("%s: env loaded from outside the module: %v", path, env)
		}
	}
}

// TestModulePathContainment_F16Junction records finding F16 (design D7 of
// harden-formal-instruments, outcome b, windows-latest run 43657442894): a
// junction inside a module is not ModeSymlink (Go 1.23+), so IsModule and Load
// admit a module with an inner junction, and before
// module-read-physical-containment the script read of j/secret followed it out
// of the module. The admission arm still asserts that behaviour; the
// script-read arm now asserts the denial by the physical containment check,
// with the verdict "containment pending windows-latest evidence" until F16 is
// reclassified (design D6 of module-read-physical-containment). The same run
// showed the other operations contained, so the leg asserts that too:
//   - both copies (modulecache.CopyModuleDir, provision.CopyDir) succeed and
//     neither contains the outside sentinel;
//   - ComputeModuleHash succeeds and does not change when only the outside
//     sentinel does.
//
// An error from a copy or a hash fails the leg, so the recorded "contained"
// verdict cannot silently mean "failed" (harden-formal-instruments decision 30).
//
// Junctions are required on Windows and skipped elsewhere through the
// capability policy.
func TestModulePathContainment_F16Junction(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapJunction)
	const sentinel = "OUTSIDE-SECRET"
	spec := fstree.Spec{RootAtom: "root", Nodes: []fstree.Node{
		{Atom: "mod", Parent: "root", Name: "m0.invowkmod", Kind: fstree.KindDir, ModuleRoot: true},
		{Atom: "out", Parent: "root", Name: "outside", Kind: fstree.KindDir},
		{Atom: "secret", Parent: "out", Name: "secret", Kind: fstree.KindFile, Content: sentinel},
		{Atom: "j", Parent: "mod", Name: "j", Kind: fstree.KindJunction, Target: "out"},
	}}
	b, missing := fstree.Materialize(t, spec, fstree.Probe(t))
	if missing != "" {
		t.Fatalf("capability %q missing after Require", string(missing))
	}
	mod := b.Path["mod"]

	mp := types.FilesystemPath(mod)
	if !invowkmod.IsModule(mp) {
		t.Fatal("F16 did not reproduce: IsModule rejected a module with an inner junction")
	}
	if _, err := invowkmod.Load(mp); err != nil {
		t.Fatalf("F16 did not reproduce: Load rejected a module with an inner junction: %v", err)
	}
	// Script-read arm: with the physical containment check (module-read-
	// physical-containment), os.Root rejects the path through the junction, so
	// the read is denied. Recorded verdict: containment pending windows-latest
	// evidence (the model still reports the escape until F16 is reclassified).
	content, err := scriptRead(mod, "j/secret")
	if !errors.Is(err, invowkfile.ErrScriptPathTraversal) {
		t.Errorf("script-read arm: the read of j/secret through the junction: err = %v, want ErrScriptPathTraversal", err)
	}
	if strings.Contains(content, sentinel) {
		t.Errorf("script-read arm: the read of j/secret returned the outside sentinel")
	}

	scratch := t.TempDir()
	copies := map[string]func(src, dst string) error{
		"modulecache.CopyModuleDir": func(src, dst string) error {
			return modulecache.CopyModuleDir(types.FilesystemPath(src), types.FilesystemPath(dst))
		},
		"provision.CopyDir": provision.CopyDir,
	}
	for name, copyDir := range copies {
		dst := filepath.Join(scratch, strings.ReplaceAll(name, ".", "-"))
		if copyErr := copyDir(mod, dst); copyErr != nil {
			t.Errorf("%s failed on a module with an inner junction: %v", name, copyErr)
			continue
		}
		if copiedOutsideContent(dst, map[string]bool{sentinel: true}) {
			t.Errorf("%s copied the outside sentinel through the junction (contained in run 43657442894)", name)
		}
	}

	before, err := invowkmod.ComputeModuleHash(mod)
	if err != nil {
		t.Fatalf("ComputeModuleHash failed on a module with an inner junction: %v", err)
	}
	if writeErr := os.WriteFile(b.Path["secret"], []byte(sentinel+"-changed"), 0o644); writeErr != nil {
		t.Fatalf("change sentinel: %v", writeErr)
	}
	after, err := invowkmod.ComputeModuleHash(mod)
	if err != nil {
		t.Fatalf("ComputeModuleHash failed after an outside-only change: %v", err)
	}
	if before != after {
		t.Errorf("ComputeModuleHash changed when only the outside sentinel changed (%s -> %s)", before, after)
	}
}
