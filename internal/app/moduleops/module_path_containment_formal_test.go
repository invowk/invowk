// SPDX-License-Identifier: MPL-2.0

package moduleops

import (
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

// TestModulePathContainment_F13VendoredSymlink records finding F13: a module's
// script.file into invowk_modules/ reaches a symlink Load never scans, and the
// read follows it outside the module. This asserts today's behaviour; the fix
// (physical containment) inverts it in a later change.
func TestModulePathContainment_F13VendoredSymlink(t *testing.T) {
	t.Parallel()
	mod, secret := vendoredSymlinkModule(t)

	sfp := invowkfile.ScriptFilePath("invowk_modules/x")
	impl := invowkfile.Implementation{Script: invowkfile.ImplementationScript{File: &sfp}}
	content, err := impl.ResolveScriptWithFSAndModule("", invowkfile.FilesystemPath(mod), os.ReadFile)
	if err != nil {
		t.Fatalf("F13 did not reproduce: script read errored: %v", err)
	}
	if content != secret {
		t.Fatalf("F13 did not reproduce: read %q, want the outside secret %q", content, secret)
	}

	// The same hole applies to a custom-check script file.
	cc := invowkfile.CustomCheckScript{File: &sfp}
	ccContent, ccErr := cc.ResolveWithFSAndModule(invowkfile.FilesystemPath(mod), os.ReadFile)
	if ccErr != nil || string(ccContent) != secret {
		t.Fatalf("F13 custom-check variant did not reproduce: %q %v", ccContent, ccErr)
	}
}

// TestModulePathContainment_F17EnvVendoredSymlink records finding F17: an env
// file declared as invowk_modules/x is read at runtime through the same symlink
// hole, with no containment check in LoadEnvFile.
func TestModulePathContainment_F17EnvVendoredSymlink(t *testing.T) {
	t.Parallel()
	mod, _ := vendoredSymlinkModule(t)
	// point the symlink at a real dotenv file
	secretEnv := filepath.Join(filepath.Dir(mod), "secret")
	if err := os.WriteFile(secretEnv, []byte("LEAKED=1\n"), 0o644); err != nil {
		t.Fatalf("write env: %v", err)
	}

	if err := invowkfile.ValidateEnvFilePath("invowk_modules/x"); err != nil {
		t.Fatalf("F17 precondition: ValidateEnvFilePath rejected the path: %v", err)
	}
	env := map[string]string{}
	if err := ivkruntime.LoadEnvFile(env, "invowk_modules/x", mod); err != nil {
		t.Fatalf("F17 did not reproduce: LoadEnvFile errored: %v", err)
	}
	if env["LEAKED"] != "1" {
		t.Fatalf("F17 did not reproduce: env not loaded from outside the module: %v", env)
	}
}

// TestModulePathContainment_F16Junction records finding F16 (design D7 of
// harden-formal-instruments, outcome b, windows-latest run 43657442894): a
// junction inside a module is not ModeSymlink (Go 1.23+), so IsModule and Load
// admit a module with an inner junction and the script read of j/secret
// follows it out of the module. This asserts today's behaviour; the fix (a
// junction-aware scan, findingF16JunctionScan's fix configuration
// scriptReadScanAllKinds) inverts it in a later change. The same run showed
// the other operations contained, so the leg asserts that too:
//   - neither copy (modulecache.CopyModuleDir, provision.CopyDir) contains the
//     outside sentinel;
//   - ComputeModuleHash does not change when only the outside sentinel does.
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
	sfp := invowkfile.ScriptFilePath("j/secret")
	impl := invowkfile.Implementation{Script: invowkfile.ImplementationScript{File: &sfp}}
	content, err := impl.ResolveScriptWithFSAndModule("", invowkfile.FilesystemPath(mod), os.ReadFile)
	if err != nil {
		t.Fatalf("F16 did not reproduce: the script read of j/secret errored: %v", err)
	}
	if content != sentinel {
		t.Fatalf("F16 did not reproduce: the script read of j/secret returned %q, want the outside sentinel", content)
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
		err := copyDir(mod, dst)
		if err == nil && copiedOutsideContent(dst, map[string]bool{sentinel: true}) {
			t.Errorf("%s copied the outside sentinel through the junction (contained in run 43657442894)", name)
		}
	}

	before, beforeErr := invowkmod.ComputeModuleHash(mod)
	if err := os.WriteFile(b.Path["secret"], []byte(sentinel+"-changed"), 0o644); err != nil {
		t.Fatalf("change sentinel: %v", err)
	}
	after, afterErr := invowkmod.ComputeModuleHash(mod)
	switch {
	case (beforeErr == nil) != (afterErr == nil):
		t.Errorf("ComputeModuleHash succeeds only on one side of an outside-only change (%v / %v)", beforeErr, afterErr)
	case beforeErr == nil && before != after:
		t.Errorf("ComputeModuleHash changed when only the outside sentinel changed (%s -> %s)", before, after)
	}
}
