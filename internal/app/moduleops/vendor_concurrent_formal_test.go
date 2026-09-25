// SPDX-License-Identifier: MPL-2.0

package moduleops

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/app/modulesync"
	"github.com/invowk/invowk/internal/testutil/procgate"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
	staleVendorURL      invowkmod.GitURL   = "https://github.com/user/a.git"
	staleVendorModuleID invowkmod.ModuleID = "io.example.a"
)

// lockLoadedGate is a real Resolver whose LoadDeclaredFromLock parks the
// vendoring process after it returns, before VendorModules copies anything.
type lockLoadedGate struct {
	*modulesync.Resolver
	gate *procgate.Gate
}

func (g *lockLoadedGate) LoadDeclaredFromLock(ctx context.Context, reqs []invowkmod.ModuleRef) ([]*invowkmod.ResolvedModule, error) {
	mods, err := g.Resolver.LoadDeclaredFromLock(ctx, reqs)
	g.gate.Park()
	return mods, err
}

func writeStaleVendorModule(t *testing.T, dir, invowkfile string) invowkmod.ContentHash {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "invowkmod.cue"), []byte("module: \""+string(staleVendorModuleID)+"\"\nversion: \"1.0.0\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkmod.cue) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "invowkfile.cue"), []byte(invowkfile), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkfile.cue) error = %v", err)
	}
	hash, err := invowkmod.ComputeModuleHash(dir)
	if err != nil {
		t.Fatalf("ComputeModuleHash() error = %v", err)
	}
	return hash
}

func staleVendorResolved(version invowkmod.SemVer, commit invowkmod.GitCommit, hash invowkmod.ContentHash) *invowkmod.ResolvedModule {
	return &invowkmod.ResolvedModule{
		ModuleRef:       invowkmod.ModuleRef{GitURL: staleVendorURL, Version: "^1.0.0"},
		ResolvedVersion: version,
		GitCommit:       commit,
		Namespace:       invowkmod.ModuleNamespace(string(staleVendorModuleID) + "@" + string(version)),
		CommandSourceID: invowkmod.ModuleSourceID(staleVendorModuleID),
		ModuleID:        staleVendorModuleID,
		ContentHash:     hash,
	}
}

// saveStaleVendorLock performs Resolver.Update's lock write for one entry:
// load the lock, replace the entry, save (resolver.go:216, 258, 264).
func saveStaleVendorLock(t *testing.T, lockPath string, mod *invowkmod.ResolvedModule) {
	t.Helper()
	lock, err := invowkmod.LoadLockFile(lockPath)
	if err != nil {
		t.Fatalf("LoadLockFile() error = %v", err)
	}
	lock.AddModule(mod)
	if err := lock.Save(lockPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
}

// TestConcurrentEdits_VendorFromStaleLock replays
// ConcurrentModuleEdits.proposedVendorFromStaleLock (a proposed finding
// without an id, escalated for allocation): vendor loads the lock, another
// process updates the module to a new version, and vendor then copies and
// verifies the old version against the hash it read, and returns ok. The
// vendored copy no longer matches the lock; discovery later rejects it, so the
// outcome fails closed. P1 is the real vendorDependenciesWithResolver with a
// real Resolver. P2 performs Resolver.Update's lock write directly, because
// this package cannot inject a fetcher and the real GitFetcher needs the
// network. That transcribed write takes no lock, so the fix change must route
// P2 through the real Update (with a test seam for its fetcher) before
// inverting this test. Asserts current behaviour.
func TestConcurrentEdits_VendorFromStaleLock(t *testing.T) {
	t.Parallel()
	workDir, cacheDir := t.TempDir(), t.TempDir()
	lockPath := filepath.Join(workDir, invowkmod.LockFileName)
	reqs := []invowkmod.ModuleRef{{GitURL: staleVendorURL, Version: "^1.0.0"}}

	resolver, err := modulesync.NewResolver(types.FilesystemPath(workDir), types.FilesystemPath(cacheDir))
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	// Lock a@1.0.0 and place its content where the resolver looks it up.
	v1Hash := writeStaleVendorModule(t, t.TempDir(), "cmds: {} // 1.0.0\n")
	saveStaleVendorLock(t, lockPath, staleVendorResolved("1.0.0", "1111111111111111111111111111111111111111", v1Hash))
	located, err := resolver.LoadDeclaredFromLock(t.Context(), reqs)
	if err != nil {
		t.Fatalf("LoadDeclaredFromLock() error = %v", err)
	}
	if got := writeStaleVendorModule(t, string(located[0].CachePath), "cmds: {} // 1.0.0\n"); got != v1Hash {
		t.Fatalf("cached hash = %s, want %s", got, v1Hash)
	}
	v2Hash := writeStaleVendorModule(t, t.TempDir(), "cmds: {} // 2.0.0\n")

	gate := procgate.New()
	done := procgate.Run(func() (*VendorResult, error) {
		_, result, _, vendorErr := vendorDependenciesWithResolver(t.Context(), types.FilesystemPath(workDir), reqs,
			&lockLoadedGate{Resolver: resolver, gate: gate}, false, false)
		return result, vendorErr
	})
	procgate.AwaitParked(t, gate, done)

	saveStaleVendorLock(t, lockPath, staleVendorResolved("2.0.0", "2222222222222222222222222222222222222222", v2Hash))
	gate.Open()
	o := <-done
	if o.Err != nil {
		t.Fatalf("vendorDependenciesWithResolver() error = %v", o.Err)
	}
	if len(o.Val.Vendored) != 1 {
		t.Fatalf("vendored %d modules, want 1", len(o.Val.Vendored))
	}

	vendorPath := o.Val.Vendored[0].VendorPath
	final, err := invowkmod.LoadLockFile(lockPath)
	if err != nil {
		t.Fatalf("LoadLockFile() error = %v", err)
	}
	entry := final.Modules[invowkmod.ModuleRefKey(staleVendorURL)]
	if entry.ContentHash != v2Hash {
		t.Fatalf("final lock hash = %s, want 2.0.0's %s", entry.ContentHash, v2Hash)
	}
	eval := invowkmod.EvaluateModuleContentHash(invowkmod.ModuleRefKey(staleVendorURL), staleVendorModuleID, vendorPath, entry.ContentHash)
	if eval.Status != invowkmod.VendoredHashMismatch || eval.Actual != v1Hash {
		t.Fatalf("vendored copy against the final lock = %+v, want a mismatch holding 1.0.0's %s", eval, v1Hash)
	}
}
