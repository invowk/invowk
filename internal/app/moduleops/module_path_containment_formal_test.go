// SPDX-License-Identifier: MPL-2.0

package moduleops

import (
	"os"
	"path/filepath"
	"testing"

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
	if !fstree.Probe(t).Symlink {
		t.Skip("symlink creation unavailable on this host")
	}
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
	if err := os.Symlink(secretPath, filepath.Join(mod, "invowk_modules", "x")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
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

// TestModulePathContainment_F16Junction probes the junction hypothesis (D8.8):
// a junction inside a module is not ModeSymlink (Go 1.23+), so
// inspectModuleEntry may not record it and a script read may follow it out of
// the module. Junctions exist only on Windows; elsewhere the leg is skipped and
// counted. F16 is unconfirmed, so the verdict is logged, not asserted: Windows CI
// output decides whether F16 is recorded.
func TestModulePathContainment_F16Junction(t *testing.T) {
	t.Parallel()
	if !fstree.Probe(t).Junction {
		t.Skip("junctions exist only on Windows; skipped and counted (runs in Windows CI)")
	}
	spec := fstree.Spec{RootAtom: "root", Nodes: []fstree.Node{
		{Atom: "mod", Parent: "root", Name: "m0.invowkmod", Kind: fstree.KindDir, ModuleRoot: true},
		{Atom: "out", Parent: "root", Name: "outside", Kind: fstree.KindDir},
		{Atom: "secret", Parent: "out", Name: "secret", Kind: fstree.KindFile, Content: "OUTSIDE-SECRET"},
		{Atom: "j", Parent: "mod", Name: "j", Kind: fstree.KindJunction, Target: "out"},
	}}
	b, skip := fstree.Materialize(t, spec, fstree.Probe(t))
	if skip != "" {
		t.Skipf("capability %q unavailable", skip)
	}
	mod := b.Path["mod"]
	_, loadErr := invowkmod.Load(types.FilesystemPath(mod))
	t.Logf("F16 probe: Load admits a module with an inner junction: %v (err %v)", loadErr == nil, loadErr)
	if loadErr != nil {
		return
	}
	sfp := invowkfile.ScriptFilePath("j/secret")
	impl := invowkfile.Implementation{Script: invowkfile.ImplementationScript{File: &sfp}}
	content, err := impl.ResolveScriptWithFSAndModule("", invowkfile.FilesystemPath(mod), os.ReadFile)
	t.Logf("F16 probe: script read through the junction escaped the module: %v (err %v)", content == "OUTSIDE-SECRET", err)
}
