// SPDX-License-Identifier: MPL-2.0

package moduleops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/app/modulesync"
	"github.com/invowk/invowk/internal/modulelock"
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
// load the lock, replace the entry, save (resolver.go:281, 323, 329).
func saveStaleVendorLock(t *testing.T, lockPath string, mod *invowkmod.ResolvedModule) {
	t.Helper()
	if err := writeStaleVendorLock(lockPath, mod); err != nil {
		t.Fatal(err)
	}
}

// writeStaleVendorLock is saveStaleVendorLock for a goroutine: it returns
// the error instead of failing the test.
func writeStaleVendorLock(lockPath string, mod *invowkmod.ResolvedModule) error {
	lock, err := invowkmod.LoadLockFile(lockPath)
	if err != nil {
		return fmt.Errorf("LoadLockFile() error = %w", err)
	}
	lock.AddModule(mod)
	if err := lock.Save(lockPath); err != nil {
		return fmt.Errorf("Save() error = %w", err)
	}
	return nil
}

// TestConcurrentEdits_VendorFromStaleLock replays the vendor case of the
// concurrent-edit model, fixed by the project lock:
// ConcurrentModuleEdits.vendorSyncProjectLock passes, and the pre-fix code
// (vendor loads the lock, another process updates the module, and vendor then
// copies the old version) is kept as
// ConcurrentModuleEdits.mutantPreFixVendorFromStaleLock. P1 is the real
// vendorDependenciesWithResolver on a real Resolver with its own FileLocker,
// parked after LoadDeclaredFromLock while it holds the project lock. P2 runs
// in a goroutine: it takes the project lock through LockProject of a second
// Resolver on the same project and cache root, then performs
// Resolver.Update's lock write by transcription (saveStaleVendorLock's
// LockFile.Save), because this package cannot inject a fetcher and the real
// GitFetcher needs the network; Update's own body is replayed by the
// F8Update tests in modulesync, so this replay checks vendor's lock window
// under the real lock Update takes. It asserts the fixed behaviour: P2 waits,
// its write lands only after the vendor's copies are complete, and the
// vendored copy matches the lock the vendor read.
func TestConcurrentEdits_VendorFromStaleLock(t *testing.T) {
	t.Parallel()
	workDir, cacheDir := t.TempDir(), t.TempDir()
	lockPath := filepath.Join(workDir, invowkmod.LockFileName)
	invowkmodPath := filepath.Join(workDir, "invowkmod.cue")
	if err := os.WriteFile(invowkmodPath, []byte("module: \"io.example.root\"\nversion: \"1.0.0\"\nrequires: [\n\t{\n\t\tgit_url: \""+string(staleVendorURL)+"\"\n\t\tversion: \"^1.0.0\"\n\t},\n]\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkmod.cue) error = %v", err)
	}
	reqs := []invowkmod.ModuleRef{{GitURL: staleVendorURL, Version: "^1.0.0"}}
	newResolver := func(onWait func(modulesync.LockWait)) *modulesync.Resolver {
		t.Helper()
		locks := modulesync.EditLocks{Locker: modulelock.NewFileLocker(), OnWait: onWait, CacheDir: types.FilesystemPath(cacheDir)}
		resolver, err := modulesync.NewResolver(types.FilesystemPath(workDir), types.FilesystemPath(cacheDir), modulesync.WithEditLocks(locks))
		if err != nil {
			t.Fatalf("NewResolver() error = %v", err)
		}
		return resolver
	}

	resolver := newResolver(nil)
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
	vendored := filepath.Join(workDir, VendoredModulesDir, string(staleVendorModuleID)+".invowkmod")

	gate := procgate.New()
	openGate := sync.OnceFunc(gate.Open)
	t.Cleanup(openGate)
	done := procgate.Run(func() (*VendorResult, error) {
		_, result, _, vendorErr := vendorDependenciesWithResolver(t.Context(), types.FilesystemPath(workDir), types.FilesystemPath(invowkmodPath),
			&lockLoadedGate{Resolver: resolver, gate: gate}, false, false)
		return result, vendorErr
	})
	procgate.AwaitParked(t, gate, done)

	lockBefore, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("ReadFile(lock) error = %v", err)
	}
	waited := make(chan struct{})
	signal := sync.OnceFunc(func() { close(waited) })
	p2 := newResolver(func(modulesync.LockWait) { signal() })
	var copyAtWrite invowkmod.ContentHash
	p2Done := procgate.Run(func() (struct{}, error) {
		unlock, lockErr := p2.LockProject(t.Context())
		if lockErr != nil {
			return struct{}{}, lockErr
		}
		hash, hashErr := invowkmod.ComputeModuleHash(vendored)
		if hashErr != nil {
			return struct{}{}, errors.Join(fmt.Errorf("vendored copy when P2 got the lock: %w", hashErr), unlock())
		}
		copyAtWrite = hash
		writeErr := writeStaleVendorLock(lockPath, staleVendorResolved("2.0.0", "2222222222222222222222222222222222222222", v2Hash))
		return struct{}{}, errors.Join(writeErr, unlock())
	})
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	select {
	case <-waited:
	case o := <-p2Done:
		t.Fatalf("P2 finished without waiting for the lock (error %v)", o.Err)
	case <-deadline.C:
		t.Fatal("P2 did not wait for the lock before the deadline")
	}
	if lockNow, readErr := os.ReadFile(lockPath); readErr != nil || !bytes.Equal(lockNow, lockBefore) {
		t.Fatalf("P2 wrote the lock file while the vendor held the project lock (read error %v)", readErr)
	}
	openGate()
	o := <-done
	if o.Err != nil {
		t.Fatalf("vendorDependenciesWithResolver() error = %v", o.Err)
	}
	if p2o := <-p2Done; p2o.Err != nil {
		t.Fatalf("P2 error = %v", p2o.Err)
	}
	if len(o.Val.Vendored) != 1 {
		t.Fatalf("vendored %d modules, want 1", len(o.Val.Vendored))
	}
	if copyAtWrite != v1Hash {
		t.Fatalf("vendored copy when P2 wrote = %s, want the complete 1.0.0 copy %s", copyAtWrite, v1Hash)
	}

	final, err := invowkmod.LoadLockFile(lockPath)
	if err != nil {
		t.Fatalf("LoadLockFile() error = %v", err)
	}
	if entry := final.Modules[invowkmod.ModuleRefKey(staleVendorURL)]; entry.ContentHash != v2Hash {
		t.Fatalf("final lock hash = %s, want P2's 2.0.0 %s", entry.ContentHash, v2Hash)
	}
	eval := invowkmod.EvaluateModuleContentHash(invowkmod.ModuleRefKey(staleVendorURL), staleVendorModuleID, o.Val.Vendored[0].VendorPath, v1Hash)
	if eval.Status != invowkmod.VendoredHashMatched {
		t.Fatalf("vendored copy against the lock the vendor read = %+v, want matched", eval)
	}
}
