// SPDX-License-Identifier: MPL-2.0

package moduleops

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/app/modulesync"
	"github.com/invowk/invowk/internal/modulelock"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const vendorLockRootInvowkmod = "module: \"io.example.root\"\nversion: \"1.0.0\"\n"

type (
	// vendorRecordingLocker is a fake modulesync.EditLocker for the vendor
	// lock-window tests: it records each Lock and unlock, can fail a Lock,
	// rewrite files before it grants one, or capture state at unlock.
	vendorRecordingLocker struct {
		lockErr     func(path types.FilesystemPath) error
		beforeGrant func()
		atUnlock    func()

		mu     sync.Mutex
		events []string
	}

	// vendorProject is a project whose module a@1.0.0 is locked and cached,
	// with invowkmod.cue and lock contents for the vendor lock-window rows.
	vendorProject struct {
		workDir   string
		cacheRoot string
		hash      invowkmod.ContentHash
		// declared is the invowkmod.cue requiring a; locked is the lock
		// holding it.
		declared []byte
		locked   []byte
	}
)

func (l *vendorRecordingLocker) Lock(_ context.Context, path types.FilesystemPath, _ func()) (func() error, error) {
	l.record("lock " + vendorLockKind(path))
	if l.lockErr != nil {
		if err := l.lockErr(path); err != nil {
			return nil, err
		}
	}
	if l.beforeGrant != nil {
		l.beforeGrant()
	}
	return func() error {
		if l.atUnlock != nil {
			l.atUnlock()
		}
		l.record("unlock " + vendorLockKind(path))
		return nil
	}, nil
}

func (l *vendorRecordingLocker) PhysicalDir(dir types.FilesystemPath) (types.FilesystemPath, error) {
	return modulelock.NewFileLocker().PhysicalDir(dir)
}

func (l *vendorRecordingLocker) record(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func vendorLockKind(path types.FilesystemPath) string {
	kind, _, _ := strings.Cut(filepath.Base(string(path)), "-")
	return kind
}

// newVendorProject locks and caches a@1.0.0 for a project whose files start
// as an invowkmod.cue without requires and no lock file.
func newVendorProject(t *testing.T) *vendorProject {
	t.Helper()
	p := &vendorProject{workDir: t.TempDir(), cacheRoot: t.TempDir()}
	lockPath := filepath.Join(p.workDir, invowkmod.LockFileName)
	p.hash = writeStaleVendorModule(t, t.TempDir(), "cmds: {} // 1.0.0\n")
	saveStaleVendorLock(t, lockPath, staleVendorResolved("1.0.0", "1111111111111111111111111111111111111111", p.hash))
	resolver, err := modulesync.NewResolver(types.FilesystemPath(p.workDir), types.FilesystemPath(p.cacheRoot))
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	located, err := resolver.LoadDeclaredFromLock(t.Context(), []invowkmod.ModuleRef{{GitURL: staleVendorURL, Version: "^1.0.0"}})
	if err != nil {
		t.Fatalf("LoadDeclaredFromLock() error = %v", err)
	}
	writeStaleVendorModule(t, string(located[0].CachePath), "cmds: {} // 1.0.0\n")
	if p.locked, err = os.ReadFile(lockPath); err != nil {
		t.Fatalf("ReadFile(lock) error = %v", err)
	}
	if err := os.Remove(lockPath); err != nil {
		t.Fatalf("Remove(lock) error = %v", err)
	}
	p.declared = []byte(vendorLockRootInvowkmod + "requires: [\n\t{\n\t\tgit_url: \"" + string(staleVendorURL) + "\"\n\t\tversion: \"^1.0.0\"\n\t},\n]\n")
	if err := os.WriteFile(p.invowkmodPath(), []byte(vendorLockRootInvowkmod), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkmod.cue) error = %v", err)
	}
	return p
}

func (p *vendorProject) invowkmodPath() string { return filepath.Join(p.workDir, "invowkmod.cue") }

func (p *vendorProject) lockPath() string { return filepath.Join(p.workDir, invowkmod.LockFileName) }

// grant writes the declared invowkmod.cue and the lock: the state the fake
// grants the project lock on.
func (p *vendorProject) grant() error {
	if err := os.WriteFile(p.invowkmodPath(), p.declared, 0o644); err != nil {
		return err
	}
	return os.WriteFile(p.lockPath(), p.locked, 0o644)
}

func (p *vendorProject) files(t *testing.T) [2][]byte {
	t.Helper()
	var out [2][]byte
	for i, path := range []string{p.invowkmodPath(), p.lockPath()} {
		data, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("ReadFile(%s) error = %v", path, err)
		}
		out[i] = data
	}
	return out
}

func (p *vendorProject) vendoredHash() (invowkmod.ContentHash, error) {
	return invowkmod.ComputeModuleHash(filepath.Join(p.workDir, VendoredModulesDir, string(staleVendorModuleID)+".invowkmod"))
}

func (p *vendorProject) resolver(t *testing.T, locker modulesync.EditLocker) *modulesync.Resolver {
	t.Helper()
	opts := []modulesync.ResolverOption{}
	if locker != nil {
		opts = append(opts, modulesync.WithEditLocks(modulesync.EditLocks{Locker: locker, CacheDir: types.FilesystemPath(p.cacheRoot)}))
	}
	resolver, err := modulesync.NewResolver(types.FilesystemPath(p.workDir), types.FilesystemPath(p.cacheRoot), opts...)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}
	return resolver
}

func (p *vendorProject) vendor(t *testing.T, resolver vendorDependencyResolver) ([]invowkmod.ModuleRef, *VendorResult, error) {
	t.Helper()
	requirements, result, _, err := vendorDependenciesWithResolver(t.Context(), types.FilesystemPath(p.workDir), types.FilesystemPath(p.invowkmodPath()), resolver, false, false)
	return requirements, result, err
}

// TestVendorDependenciesHoldsProjectLock is the vendor row of the lock-window
// test (design D4): vendor reads invowkmod.cue and the lock only after the
// project lock is granted, its copies into invowk_modules/ are complete
// before the release, nothing changes after it, and a cancelled or timed-out
// wait fails with both files byte-identical.
func TestVendorDependenciesHoldsProjectLock(t *testing.T) {
	t.Parallel()

	t.Run("reads and writes inside the lock", func(t *testing.T) {
		t.Parallel()
		p := newVendorProject(t)
		var atUnlock [2][]byte
		var hashAtUnlock invowkmod.ContentHash
		var hashErr error
		locker := &vendorRecordingLocker{
			beforeGrant: func() {
				if err := p.grant(); err != nil {
					panic(fmt.Sprintf("rewrite files before the grant: %v", err))
				}
			},
			atUnlock: func() {
				atUnlock = p.files(t)
				hashAtUnlock, hashErr = p.vendoredHash()
			},
		}

		requirements, result, err := p.vendor(t, p.resolver(t, locker))
		if err != nil {
			t.Fatalf("vendorDependenciesWithResolver() error = %v", err)
		}
		if len(requirements) != 1 || result == nil || len(result.Vendored) != 1 {
			t.Fatalf("vendored %+v for %d requirements, want the one only the granted invowkmod.cue declares", result, len(requirements))
		}
		if hashErr != nil || hashAtUnlock != p.hash {
			t.Fatalf("vendored copy at the release = %s (%v), want the complete copy %s", hashAtUnlock, hashErr, p.hash)
		}
		if after := p.files(t); !bytes.Equal(after[0], atUnlock[0]) || !bytes.Equal(after[1], atUnlock[1]) {
			t.Fatal("invowkmod.cue or the lock file changed after the release")
		}
		if !slices.Equal(locker.events, []string{"lock project", "unlock project"}) {
			t.Fatalf("lock events = %v, want one project lock and its release", locker.events)
		}
	})

	for _, wait := range []struct {
		name  string
		err   func(path types.FilesystemPath) error
		check func(t *testing.T, err error, path types.FilesystemPath)
	}{
		{
			name: "cancelled wait",
			err: func(path types.FilesystemPath) error {
				return fmt.Errorf("waiting for lock file %s: %w", path, context.Canceled)
			},
			check: func(t *testing.T, err error, _ types.FilesystemPath) {
				t.Helper()
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("error = %v, want it to wrap context.Canceled", err)
				}
			},
		},
		{
			name: "timed-out wait",
			err: func(path types.FilesystemPath) error {
				return &modulelock.TimeoutError{Path: path, Timeout: time.Second}
			},
			check: func(t *testing.T, err error, path types.FilesystemPath) {
				t.Helper()
				var timeout *modulelock.TimeoutError
				if !errors.As(err, &timeout) || !strings.Contains(err.Error(), string(path)) || vendorLockKind(path) != "project" {
					t.Fatalf("error = %v, want a *modulelock.TimeoutError naming the project lock file %s", err, path)
				}
			},
		},
	} {
		t.Run(wait.name, func(t *testing.T) {
			t.Parallel()
			p := newVendorProject(t)
			if err := p.grant(); err != nil {
				t.Fatalf("write files: %v", err)
			}
			before := p.files(t)
			var lockPath types.FilesystemPath
			locker := &vendorRecordingLocker{lockErr: func(path types.FilesystemPath) error { lockPath = path; return wait.err(path) }}

			_, _, err := p.vendor(t, p.resolver(t, locker))
			if err == nil {
				t.Fatal("vendorDependenciesWithResolver() error = nil, want the lock wait's error")
			}
			wait.check(t, err, lockPath)
			if after := p.files(t); !bytes.Equal(after[0], before[0]) || !bytes.Equal(after[1], before[1]) {
				t.Fatal("invowkmod.cue or the lock file changed after a failed lock wait")
			}
			if _, statErr := os.Stat(filepath.Join(p.workDir, VendoredModulesDir)); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("invowk_modules stat error = %v, want nothing vendored", statErr)
			}
		})
	}
}

// TestVendorDependenciesRequiresLocker checks design D2 for vendor: both
// VendorDependencies and vendorDependenciesWithResolver refuse to run without
// a locker, before they read either file or create a cache directory.
func TestVendorDependenciesRequiresLocker(t *testing.T) {
	t.Parallel()

	t.Run("VendorDependencies", func(t *testing.T) {
		t.Parallel()
		p := newVendorProject(t)
		if err := p.grant(); err != nil {
			t.Fatalf("write files: %v", err)
		}
		before := p.files(t)
		cacheRoot := filepath.Join(t.TempDir(), "modules")
		_, _, _, err := VendorDependencies(t.Context(), modulesync.EditLocks{CacheDir: types.FilesystemPath(cacheRoot)}, types.FilesystemPath(p.workDir), false, false)
		if !errors.Is(err, modulesync.ErrEditLockerRequired) {
			t.Fatalf("VendorDependencies() error = %v, want ErrEditLockerRequired", err)
		}
		if after := p.files(t); !bytes.Equal(after[0], before[0]) || !bytes.Equal(after[1], before[1]) {
			t.Fatal("files changed")
		}
		if _, statErr := os.Stat(cacheRoot); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("cache root stat error = %v, want not exist", statErr)
		}
	})
	t.Run("vendorDependenciesWithResolver", func(t *testing.T) {
		t.Parallel()
		p := newVendorProject(t)
		if err := p.grant(); err != nil {
			t.Fatalf("write files: %v", err)
		}
		before := p.files(t)
		if _, _, err := p.vendor(t, p.resolver(t, nil)); !errors.Is(err, modulesync.ErrEditLockerRequired) {
			t.Fatalf("vendorDependenciesWithResolver() error = %v, want ErrEditLockerRequired", err)
		}
		if after := p.files(t); !bytes.Equal(after[0], before[0]) || !bytes.Equal(after[1], before[1]) {
			t.Fatal("files changed")
		}
		if _, statErr := os.Stat(filepath.Join(p.cacheRoot, "locks")); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("locks stat error = %v, want not exist", statErr)
		}
	})
}

// TestVendorDependenciesWithoutRequirementsCreatesOnlyProjectLock checks
// design D5 for an empty vendor from a cache root that does not exist: it
// creates the cache root, locks/ and the project lock file, and nothing else.
func TestVendorDependenciesWithoutRequirementsCreatesOnlyProjectLock(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(workDir, "invowkmod.cue"), []byte(vendorLockRootInvowkmod), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkmod.cue) error = %v", err)
	}
	cacheRoot := filepath.Join(t.TempDir(), "modules")

	requirements, result, _, err := VendorDependencies(t.Context(), modulesync.EditLocks{Locker: modulelock.NewFileLocker(), CacheDir: types.FilesystemPath(cacheRoot)}, types.FilesystemPath(workDir), false, false)
	if err != nil || len(requirements) != 0 || result != nil {
		t.Fatalf("VendorDependencies() = %v, %v, %v; want nothing to vendor", requirements, result, err)
	}

	physical, err := modulelock.NewFileLocker().PhysicalDir(types.FilesystemPath(workDir))
	if err != nil {
		t.Fatalf("PhysicalDir() error = %v", err)
	}
	sum := sha256.Sum256([]byte(physical))
	want := []string{"locks", "locks/project-" + hex.EncodeToString(sum[:]) + ".lock"}
	var got []string
	walkErr := filepath.WalkDir(cacheRoot, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if rel, relErr := filepath.Rel(cacheRoot, path); relErr == nil && rel != "." {
			got = append(got, filepath.ToSlash(rel))
		}
		return nil
	})
	if walkErr != nil || !slices.Equal(got, want) {
		t.Fatalf("cache root holds %v (%v), want only %v", got, walkErr, want)
	}
}
