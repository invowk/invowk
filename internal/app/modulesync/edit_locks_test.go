// SPDX-License-Identifier: MPL-2.0

package modulesync

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

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/modulelock"
	"github.com/invowk/invowk/internal/testutil/fstree"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

type (
	// recordingLocker is a fake EditLocker: it records each Lock and unlock
	// by kind ("project" or "source", from the lock file name), can fail a
	// Lock, rewrite files before it grants one, or capture files at unlock.
	// With inner set it also takes the real lock.
	recordingLocker struct {
		inner       EditLocker
		lockErr     func(path types.FilesystemPath) error
		beforeGrant func(path types.FilesystemPath)
		atUnlock    func(path types.FilesystemPath)

		mu     sync.Mutex
		events []string
		paths  []types.FilesystemPath
	}

	// lockWindowFiles is the content of invowkmod.cue and the lock file;
	// a nil lock means no lock file.
	lockWindowFiles struct {
		invowkmod []byte
		lock      []byte
	}

	// lockWindowCase is one locking use case of TestModuleEditsHoldProjectLock:
	// it starts from before, the fake rewrites the files to granted before it
	// grants the project lock, run runs the command, and check asserts that the
	// result derives from granted.
	lockWindowCase struct {
		name    string
		before  func(t *testing.T, source string) lockWindowFiles
		granted func(t *testing.T, source string) lockWindowFiles
		run     func(t *testing.T, r *Resolver, invowkmodPath types.FilesystemPath, granted lockWindowFiles) error
	}
)

func (l *recordingLocker) Lock(ctx context.Context, path types.FilesystemPath, onBusy func()) (func() error, error) {
	kind := lockKindOf(path)
	l.record("lock "+kind, path)
	if l.lockErr != nil {
		if err := l.lockErr(path); err != nil {
			return nil, err
		}
	}
	if l.beforeGrant != nil {
		l.beforeGrant(path)
	}
	var inner func() error
	if l.inner != nil {
		var err error
		if inner, err = l.inner.Lock(ctx, path, onBusy); err != nil {
			return nil, err
		}
	}
	return func() error {
		if l.atUnlock != nil {
			l.atUnlock(path)
		}
		l.record("unlock "+kind, path)
		if inner != nil {
			return inner()
		}
		return nil
	}, nil
}

func (l *recordingLocker) PhysicalDir(dir types.FilesystemPath) (types.FilesystemPath, error) {
	if l.inner != nil {
		return l.inner.PhysicalDir(dir)
	}
	abs, err := filepath.Abs(string(dir))
	if err != nil {
		return "", fmt.Errorf("abs: %w", err)
	}
	physical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("eval symlinks: %w", err)
	}
	return types.FilesystemPath(physical), nil
}

func (l *recordingLocker) record(event string, path types.FilesystemPath) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
	l.paths = append(l.paths, path)
}

func (l *recordingLocker) recorded() (events []string, paths []types.FilesystemPath) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.events), slices.Clone(l.paths)
}

func lockKindOf(path types.FilesystemPath) string {
	kind, _, _ := strings.Cut(filepath.Base(string(path)), "-")
	return kind
}

// testEditLocks returns edit locks with a real FileLocker on a temporary
// cache root, so no locking test touches the developer's ~/.invowk/modules.
func testEditLocks(t *testing.T) EditLocks {
	t.Helper()
	return EditLocks{Locker: modulelock.NewFileLocker(), CacheDir: types.FilesystemPath(t.TempDir())}
}

// withEditLocks gives r a real FileLocker on its own cache root.
func withEditLocks(r *Resolver) *Resolver {
	r.locks = &editLocks{locks: EditLocks{Locker: modulelock.NewFileLocker(), CacheDir: r.cacheDir}, cacheDir: r.cacheDir}
	return r
}

// withRecordingLocker gives r the recording locker l on its own cache root.
func withRecordingLocker(r *Resolver, l *recordingLocker) *Resolver {
	r.locks = &editLocks{locks: EditLocks{Locker: l, CacheDir: r.cacheDir}, cacheDir: r.cacheDir}
	return r
}

// projectLockName is the lock file name LockProject uses for dir.
func projectLockName(t *testing.T, dir string) string {
	t.Helper()
	physical, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%s) error = %v", dir, err)
	}
	abs, err := filepath.Abs(physical)
	if err != nil {
		t.Fatalf("Abs(%s) error = %v", physical, err)
	}
	sum := sha256.Sum256([]byte(abs))
	return "project-" + hex.EncodeToString(sum[:]) + ".lock"
}

// assertOnlyProjectLock checks that cacheRoot holds nothing but locks/ with
// the project lock file of projectDir.
func assertOnlyProjectLock(t *testing.T, cacheRoot, projectDir string) {
	t.Helper()
	var got []string
	err := filepath.WalkDir(cacheRoot, func(path string, _ os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, relErr := filepath.Rel(cacheRoot, path)
		if relErr != nil {
			return fmt.Errorf("rel: %w", relErr)
		}
		if rel != "." {
			got = append(got, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk cache root %s: %v", cacheRoot, err)
	}
	want := []string{locksDirName, locksDirName + "/" + projectLockName(t, projectDir)}
	if !slices.Equal(got, want) {
		t.Fatalf("cache root holds %v, want only %v", got, want)
	}
}

func readLockWindowFiles(t *testing.T, workDir string) lockWindowFiles {
	t.Helper()
	invowkmodData, err := os.ReadFile(filepath.Join(workDir, "invowkmod.cue"))
	if err != nil {
		t.Fatalf("ReadFile(invowkmod.cue) error = %v", err)
	}
	lockData, err := os.ReadFile(filepath.Join(workDir, LockFileName))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ReadFile(lock) error = %v", err)
	}
	return lockWindowFiles{invowkmod: invowkmodData, lock: lockData}
}

func writeLockWindowFiles(workDir string, files lockWindowFiles) error {
	if err := os.WriteFile(filepath.Join(workDir, "invowkmod.cue"), files.invowkmod, 0o644); err != nil {
		return fmt.Errorf("write invowkmod.cue: %w", err)
	}
	lockPath := filepath.Join(workDir, LockFileName)
	if files.lock == nil {
		if err := os.Remove(lockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove lock: %w", err)
		}
		return nil
	}
	if err := os.WriteFile(lockPath, files.lock, 0o644); err != nil {
		return fmt.Errorf("write lock: %w", err)
	}
	return nil
}

func assertLockWindowFiles(t *testing.T, what string, got, want lockWindowFiles) {
	t.Helper()
	if !bytes.Equal(got.invowkmod, want.invowkmod) || !bytes.Equal(got.lock, want.lock) || (got.lock == nil) != (want.lock == nil) {
		t.Fatalf("%s:\ninvowkmod.cue:\n%s\nwant:\n%s\nlock:\n%s\nwant:\n%s", what, got.invowkmod, want.invowkmod, got.lock, want.lock)
	}
}

// buildLockWindowFiles builds, through the real code in a scratch project,
// an invowkmod.cue requiring requires and a lock holding locked (no lock
// file when locked is nil). Every module is served from source.
func buildLockWindowFiles(t *testing.T, source string, requires, locked []GitURL) lockWindowFiles {
	t.Helper()
	dir := t.TempDir()
	invowkmodPath := types.FilesystemPath(filepath.Join(dir, "invowkmod.cue"))
	if err := os.WriteFile(string(invowkmodPath), []byte("module: \"io.example.root\"\nversion: \"1.0.0\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkmod.cue) error = %v", err)
	}
	for _, url := range requires {
		if err := invowkmod.AddRequirement(invowkmodPath, ModuleRef{GitURL: url, Version: "^1.0.0"}); err != nil {
			t.Fatalf("AddRequirement(%s) error = %v", url, err)
		}
	}
	if locked != nil {
		if err := invowkmod.NewLockFile().Save(filepath.Join(dir, LockFileName)); err != nil {
			t.Fatalf("Save(empty lock) error = %v", err)
		}
	}
	for _, url := range locked {
		r, err := newResolverWithFetcher(types.FilesystemPath(dir), types.FilesystemPath(t.TempDir()), &fakeModuleFetcher{repoPath: types.FilesystemPath(source), listVersions: []SemVer{"1.0.0"}})
		if err != nil {
			t.Fatalf("newResolverWithFetcher() error = %v", err)
		}
		if _, _, err := r.Add(t.Context(), ModuleRef{GitURL: url, Version: "^1.0.0"}); err != nil {
			t.Fatalf("Add(%s) error = %v", url, err)
		}
	}
	return readLockWindowFiles(t, dir)
}

func lockWindowCases() []lockWindowCase {
	none := func(t *testing.T, source string) lockWindowFiles {
		t.Helper()
		return buildLockWindowFiles(t, source, nil, nil)
	}
	empty := func(t *testing.T, source string) lockWindowFiles {
		t.Helper()
		return buildLockWindowFiles(t, source, nil, []GitURL{})
	}
	requiresA := func(t *testing.T, source string) lockWindowFiles {
		t.Helper()
		return buildLockWindowFiles(t, source, []GitURL{concurrentURLA}, nil)
	}
	bothA := func(t *testing.T, source string) lockWindowFiles {
		t.Helper()
		return buildLockWindowFiles(t, source, []GitURL{concurrentURLA}, []GitURL{concurrentURLA})
	}
	return []lockWindowCase{
		{
			// The granted invowkmod.cue already requires a: the edit fails as a
			// duplicate and the rollback must restore the granted files, which
			// it snapshotted under the lock.
			name:   "add",
			before: empty,
			granted: func(t *testing.T, source string) lockWindowFiles {
				t.Helper()
				return buildLockWindowFiles(t, source, []GitURL{concurrentURLA}, []GitURL{})
			},
			run: func(t *testing.T, r *Resolver, invowkmodPath types.FilesystemPath, granted lockWindowFiles) error {
				t.Helper()
				result, err := r.AddModuleDependency(t.Context(), invowkmodPath, ModuleRef{GitURL: concurrentURLA, Version: "^1.0.0"})
				if err != nil {
					return err
				}
				if result.Declaration().Rollback() != DeclarationRollbackComplete || !errors.Is(result.Declaration().Err(), invowkmod.ErrModuleAlreadyExists) {
					t.Fatalf("Declaration = %s, %v; want a complete rollback of a duplicate", result.Declaration().Rollback(), result.Declaration().Err())
				}
				assertLockWindowFiles(t, "after the rolled-back add", readLockWindowFiles(t, string(r.WorkingDir())), granted)
				return nil
			},
		},
		{
			name:    "remove",
			before:  empty,
			granted: bothA,
			run: func(t *testing.T, r *Resolver, invowkmodPath types.FilesystemPath, _ lockWindowFiles) error {
				t.Helper()
				result, err := r.RemoveModuleDependency(t.Context(), invowkmodPath, string(concurrentURLA))
				if err != nil {
					return err
				}
				if len(result.Removed()) != 1 {
					t.Fatalf("Removed = %d, want the entry only the granted lock holds", len(result.Removed()))
				}
				assertKeys(t, "lock", lockKeys(t, string(r.WorkingDir())))
				return nil
			},
		},
		{
			name:    "sync",
			before:  none,
			granted: requiresA,
			run: func(t *testing.T, r *Resolver, invowkmodPath types.FilesystemPath, _ lockWindowFiles) error {
				t.Helper()
				result, err := r.SyncModule(t.Context(), invowkmodPath)
				if err != nil {
					return err
				}
				if len(result.Requirements()) != 1 {
					t.Fatalf("Requirements = %d, want the one only the granted invowkmod.cue declares", len(result.Requirements()))
				}
				assertKeys(t, "lock", lockKeys(t, string(r.WorkingDir())), concurrentURLA)
				return nil
			},
		},
		{
			name:    "update",
			before:  empty,
			granted: bothA,
			run: func(t *testing.T, r *Resolver, _ types.FilesystemPath, _ lockWindowFiles) error {
				t.Helper()
				result, err := r.UpdateModule(t.Context(), "")
				if err != nil {
					return err
				}
				if len(result.Updated()) != 1 {
					t.Fatalf("Updated = %d, want the entry only the granted lock holds", len(result.Updated()))
				}
				assertKeys(t, "lock", lockKeys(t, string(r.WorkingDir())), concurrentURLA)
				return nil
			},
		},
		{
			name:    "tidy",
			before:  none,
			granted: requiresA,
			run: func(t *testing.T, r *Resolver, invowkmodPath types.FilesystemPath, _ lockWindowFiles) error {
				t.Helper()
				requirements, _, err := r.TidyModule(t.Context(), invowkmodPath)
				if err != nil {
					return err
				}
				if len(requirements) != 1 {
					t.Fatalf("requirements = %d, want the one only the granted invowkmod.cue declares", len(requirements))
				}
				return nil
			},
		},
	}
}

// TestModuleEditsHoldProjectLock checks the project-lock window of add,
// remove, sync, update and tidy (design D4), each taking the lock through
// Resolver.LockProject: nothing is read before the lock (the fake rewrites
// both files before it grants it, and the result derives from that state),
// nothing is written after the release, and a cancelled or timed-out wait
// fails the command with both files byte-identical.
func TestModuleEditsHoldProjectLock(t *testing.T) {
	t.Parallel()

	for _, tt := range lockWindowCases() {
		t.Run(tt.name+"/reads and writes inside the lock", func(t *testing.T) {
			t.Parallel()
			source := t.TempDir()
			writeSourceModule(t, source, "io.example.a")
			workDir := t.TempDir()
			if err := writeLockWindowFiles(workDir, tt.before(t, source)); err != nil {
				t.Fatalf("write initial files: %v", err)
			}
			granted := tt.granted(t, source)
			var atUnlock lockWindowFiles
			locker := &recordingLocker{
				beforeGrant: func(path types.FilesystemPath) {
					if lockKindOf(path) != "project" {
						return
					}
					if err := writeLockWindowFiles(workDir, granted); err != nil {
						panic(fmt.Sprintf("rewrite files before the grant: %v", err))
					}
				},
				atUnlock: func(path types.FilesystemPath) {
					if lockKindOf(path) == "project" {
						atUnlock = readLockWindowFiles(t, workDir)
					}
				},
			}
			r := newLockWindowResolver(t, workDir, source, locker)

			if err := tt.run(t, r, types.FilesystemPath(filepath.Join(workDir, "invowkmod.cue")), granted); err != nil {
				t.Fatalf("%s error = %v", tt.name, err)
			}
			events, paths := locker.recorded()
			if len(events) < 2 || events[0] != "lock project" || events[len(events)-1] != "unlock project" ||
				slices.Contains(events[1:len(events)-1], "lock project") || slices.Contains(events[1:len(events)-1], "unlock project") {
				t.Fatalf("lock events = %v, want one project lock around everything else (source locks only inside it)", events)
			}
			if got := filepath.Base(string(paths[0])); got != projectLockName(t, workDir) {
				t.Fatalf("project lock file = %s, want %s", got, projectLockName(t, workDir))
			}
			assertLockWindowFiles(t, "files changed after the release", readLockWindowFiles(t, workDir), atUnlock)
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
					if _, ok := errors.AsType[*modulelock.TimeoutError](err); !ok {
						t.Fatalf("error = %v, want a *modulelock.TimeoutError", err)
					}
					if !strings.Contains(err.Error(), string(path)) || lockKindOf(path) != "project" {
						t.Fatalf("error = %v, want it to name the project lock file %s", err, path)
					}
				},
			},
		} {
			t.Run(tt.name+"/"+wait.name, func(t *testing.T) {
				t.Parallel()
				source := t.TempDir()
				writeSourceModule(t, source, "io.example.a")
				workDir := t.TempDir()
				before := tt.before(t, source)
				if err := writeLockWindowFiles(workDir, before); err != nil {
					t.Fatalf("write initial files: %v", err)
				}
				var lockPath types.FilesystemPath
				locker := &recordingLocker{lockErr: func(path types.FilesystemPath) error { lockPath = path; return wait.err(path) }}
				r := newLockWindowResolver(t, workDir, source, locker)

				err := tt.run(t, r, types.FilesystemPath(filepath.Join(workDir, "invowkmod.cue")), lockWindowFiles{})
				if err == nil {
					t.Fatalf("%s error = nil, want the lock wait's error", tt.name)
				}
				wait.check(t, err, lockPath)
				assertLockWindowFiles(t, "files after a failed lock wait", readLockWindowFiles(t, workDir), before)
			})
		}
	}
}

func newLockWindowResolver(t *testing.T, workDir, source string, locker *recordingLocker) *Resolver {
	t.Helper()
	r, err := newResolverWithFetcher(types.FilesystemPath(workDir), types.FilesystemPath(t.TempDir()), &fakeModuleFetcher{repoPath: types.FilesystemPath(source), listVersions: []SemVer{"1.0.0"}})
	if err != nil {
		t.Fatalf("newResolverWithFetcher() error = %v", err)
	}
	return withRecordingLocker(r, locker)
}

// TestModuleEditUseCasesRequireLocker checks design D2: every locking
// package-level use case and Resolver method refuses to run without a
// locker, before it reads either file or creates a cache directory.
func TestModuleEditUseCasesRequireLocker(t *testing.T) {
	t.Parallel()

	ref := ModuleRef{GitURL: concurrentURLA, Version: "^1.0.0"}
	packageLevel := map[string]func(ctx context.Context, locks EditLocks, path types.FilesystemPath) error{
		"AddModuleDependency": func(ctx context.Context, locks EditLocks, path types.FilesystemPath) error {
			_, err := AddModuleDependency(ctx, locks, path, ref)
			return err
		},
		"RemoveModuleDependency": func(ctx context.Context, locks EditLocks, path types.FilesystemPath) error {
			_, err := RemoveModuleDependency(ctx, locks, path, string(concurrentURLA))
			return err
		},
		"SyncModule": func(ctx context.Context, locks EditLocks, path types.FilesystemPath) error {
			_, err := SyncModule(ctx, locks, path)
			return err
		},
		"UpdateModule": func(ctx context.Context, locks EditLocks, path types.FilesystemPath) error {
			_, err := UpdateModule(ctx, locks, path, "")
			return err
		},
		"TidyModule": func(ctx context.Context, locks EditLocks, path types.FilesystemPath) error {
			_, _, err := TidyModule(ctx, locks, path)
			return err
		},
	}
	for name, run := range packageLevel {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			workDir := t.TempDir()
			if err := writeLockWindowFiles(workDir, lockWindowFiles{invowkmod: []byte(lockWrittenRootInvowkmod)}); err != nil {
				t.Fatalf("write files: %v", err)
			}
			cacheDir := filepath.Join(t.TempDir(), "modules")
			err := run(t.Context(), EditLocks{CacheDir: types.FilesystemPath(cacheDir)}, types.FilesystemPath(filepath.Join(workDir, "invowkmod.cue")))
			if !errors.Is(err, ErrEditLockerRequired) {
				t.Fatalf("%s error = %v, want ErrEditLockerRequired", name, err)
			}
			assertLockWindowFiles(t, "files", readLockWindowFiles(t, workDir), lockWindowFiles{invowkmod: []byte(lockWrittenRootInvowkmod)})
			assertPathAbsent(t, cacheDir)
		})
	}

	methods := map[string]func(ctx context.Context, r *Resolver, path types.FilesystemPath) error{
		"Resolver.AddModuleDependency": func(ctx context.Context, r *Resolver, path types.FilesystemPath) error {
			_, err := r.AddModuleDependency(ctx, path, ref)
			return err
		},
		"Resolver.RemoveModuleDependency": func(ctx context.Context, r *Resolver, path types.FilesystemPath) error {
			_, err := r.RemoveModuleDependency(ctx, path, string(concurrentURLA))
			return err
		},
		"Resolver.SyncModule": func(ctx context.Context, r *Resolver, path types.FilesystemPath) error {
			_, err := r.SyncModule(ctx, path)
			return err
		},
		"Resolver.UpdateModule": func(ctx context.Context, r *Resolver, _ types.FilesystemPath) error {
			_, err := r.UpdateModule(ctx, "")
			return err
		},
		"Resolver.TidyModule": func(ctx context.Context, r *Resolver, path types.FilesystemPath) error {
			_, _, err := r.TidyModule(ctx, path)
			return err
		},
		"Resolver.LockProject": func(ctx context.Context, r *Resolver, _ types.FilesystemPath) error {
			_, err := r.LockProject(ctx)
			return err
		},
	}
	for name, run := range methods {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			workDir := t.TempDir()
			if err := writeLockWindowFiles(workDir, lockWindowFiles{invowkmod: []byte(lockWrittenRootInvowkmod)}); err != nil {
				t.Fatalf("write files: %v", err)
			}
			cacheDir := t.TempDir()
			r, err := newResolverWithFetcher(types.FilesystemPath(workDir), types.FilesystemPath(cacheDir), &fakeModuleFetcher{})
			if err != nil {
				t.Fatalf("newResolverWithFetcher() error = %v", err)
			}
			if err := run(t.Context(), r, types.FilesystemPath(filepath.Join(workDir, "invowkmod.cue"))); !errors.Is(err, ErrEditLockerRequired) {
				t.Fatalf("%s error = %v, want ErrEditLockerRequired", name, err)
			}
			assertLockWindowFiles(t, "files", readLockWindowFiles(t, workDir), lockWindowFiles{invowkmod: []byte(lockWrittenRootInvowkmod)})
			assertPathAbsent(t, filepath.Join(cacheDir, locksDirName))
		})
	}
}

// TestUpdateModuleWithoutLockWritesEmptyLock runs the package-level
// UpdateModule on a project with no lock file (design D2): it saves an empty
// v2 lock, and the cache root holds only locks/ and the project lock file.
func TestUpdateModuleWithoutLockWritesEmptyLock(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	invowkmodPath := writeLockWrittenRootInvowkmod(t, workDir)
	cacheRoot := filepath.Join(t.TempDir(), "modules")

	result, err := UpdateModule(t.Context(), EditLocks{Locker: modulelock.NewFileLocker(), CacheDir: types.FilesystemPath(cacheRoot)}, invowkmodPath, "")
	if err != nil {
		t.Fatalf("UpdateModule() error = %v", err)
	}
	if !result.LockWritten() || len(result.Updated()) != 0 {
		t.Fatalf("UpdateModule() = %d updated, LockWritten %t; want an empty lock written", len(result.Updated()), result.LockWritten())
	}
	lock := loadLockFileForTest(t, filepath.Join(workDir, LockFileName))
	if lock.Version != invowkmod.NewLockFile().Version || len(lock.Modules) != 0 {
		t.Fatalf("lock = version %s with %d modules, want an empty v%s lock", lock.Version, len(lock.Modules), invowkmod.NewLockFile().Version)
	}
	assertOnlyProjectLock(t, cacheRoot, workDir)
}

// TestTidyModuleWithoutRequirementsCreatesOnlyProjectLock checks design D5
// for an empty tidy from a cache root that does not exist: it creates the
// cache root, locks/ and the project lock file, and nothing else.
func TestTidyModuleWithoutRequirementsCreatesOnlyProjectLock(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	invowkmodPath := writeLockWrittenRootInvowkmod(t, workDir)
	cacheRoot := filepath.Join(t.TempDir(), "modules")

	requirements, missing, err := TidyModule(t.Context(), EditLocks{Locker: modulelock.NewFileLocker(), CacheDir: types.FilesystemPath(cacheRoot)}, invowkmodPath)
	if err != nil || len(requirements) != 0 || len(missing) != 0 {
		t.Fatalf("TidyModule() = %v, %v, %v; want nothing to tidy", requirements, missing, err)
	}
	assertOnlyProjectLock(t, cacheRoot, workDir)
	assertPathAbsent(t, filepath.Join(workDir, LockFileName))
}

// TestModuleEditLockDirectoryNotWritable checks design D5: when the lock
// directory cannot be created (the cache root is under a regular file), add,
// remove and sync fail with the actionable error and leave both files
// untouched.
func TestModuleEditLockDirectoryNotWritable(t *testing.T) {
	t.Parallel()

	runs := map[string]func(ctx context.Context, locks EditLocks, path types.FilesystemPath) error{
		"add": func(ctx context.Context, locks EditLocks, path types.FilesystemPath) error {
			_, err := AddModuleDependency(ctx, locks, path, ModuleRef{GitURL: concurrentURLA, Version: "^1.0.0"})
			return err
		},
		"remove": func(ctx context.Context, locks EditLocks, path types.FilesystemPath) error {
			_, err := RemoveModuleDependency(ctx, locks, path, string(concurrentURLA))
			return err
		},
		"sync": func(ctx context.Context, locks EditLocks, path types.FilesystemPath) error {
			_, err := SyncModule(ctx, locks, path)
			return err
		},
	}
	for name, run := range runs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			source := t.TempDir()
			writeSourceModule(t, source, "io.example.a")
			workDir := t.TempDir()
			before := buildLockWindowFiles(t, source, []GitURL{concurrentURLA}, []GitURL{concurrentURLA})
			if err := writeLockWindowFiles(workDir, before); err != nil {
				t.Fatalf("write files: %v", err)
			}
			blocker := filepath.Join(t.TempDir(), "regular-file")
			if err := os.WriteFile(blocker, []byte("not a directory"), 0o644); err != nil {
				t.Fatalf("WriteFile(blocker) error = %v", err)
			}
			cacheRoot := filepath.Join(blocker, "modules")

			err := run(t.Context(), EditLocks{Locker: modulelock.NewFileLocker(), CacheDir: types.FilesystemPath(cacheRoot)}, types.FilesystemPath(filepath.Join(workDir, "invowkmod.cue")))
			if err == nil || !strings.Contains(err.Error(), "cannot create module lock directory "+filepath.Join(cacheRoot, locksDirName)) || !strings.Contains(err.Error(), "set INVOWK_MODULES_PATH to a writable directory") {
				t.Fatalf("%s error = %v, want the actionable lock directory error", name, err)
			}
			assertLockWindowFiles(t, "files", readLockWindowFiles(t, workDir), before)
		})
	}
}

// TestReadOnlyConsumersIgnoreProjectLock checks design D4 and D7: while
// another process holds a project's lock, ListModuleDependencies,
// LoadRequirements and the discovery load that invowk cmd and validate use
// all return under a short deadline.
func TestReadOnlyConsumersIgnoreProjectLock(t *testing.T) {
	t.Parallel()

	source := t.TempDir()
	writeSourceModule(t, source, "io.example.a")
	workDir := t.TempDir()
	if err := writeLockWindowFiles(workDir, buildLockWindowFiles(t, source, []GitURL{concurrentURLA}, []GitURL{concurrentURLA})); err != nil {
		t.Fatalf("write files: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workDir, "invowkfile.cue"), []byte("cmds: []\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(invowkfile.cue) error = %v", err)
	}
	holder, err := newResolverWithFetcher(types.FilesystemPath(workDir), types.FilesystemPath(t.TempDir()), &fakeModuleFetcher{})
	if err != nil {
		t.Fatalf("newResolverWithFetcher() error = %v", err)
	}
	unlock, err := withEditLocks(holder).LockProject(t.Context())
	if err != nil {
		t.Fatalf("LockProject() error = %v", err)
	}
	t.Cleanup(func() {
		if unlockErr := unlock(); unlockErr != nil {
			t.Errorf("unlock: %v", unlockErr)
		}
	})

	invowkmodPath := types.FilesystemPath(filepath.Join(workDir, "invowkmod.cue"))
	consumers := map[string]func(ctx context.Context) error{
		"ListModuleDependencies": func(ctx context.Context) error {
			_, listErr := ListModuleDependencies(ctx, invowkmodPath)
			return listErr
		},
		"LoadRequirements": func(context.Context) error {
			_, loadErr := LoadRequirements(invowkmodPath)
			return loadErr
		},
		"discovery": func(context.Context) error {
			_, loadErr := discovery.New(config.DefaultConfig(), discovery.WithBaseDir(types.FilesystemPath(workDir)), discovery.WithCommandsDir("")).LoadAll()
			return loadErr
		},
	}
	for name, consume := range consumers {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		done := make(chan error, 1)
		go func() { done <- consume(ctx) }()
		select {
		case consumeErr := <-done:
			if consumeErr != nil {
				t.Errorf("%s error = %v", name, consumeErr)
			}
		case <-ctx.Done():
			t.Errorf("%s did not return while the project lock was held", name)
		}
		cancel()
	}
}

// TestLockProjectSameFileThroughSymlink checks design D1: a resolver on the
// project's real path and one on a symbolic link to it take one lock file.
func TestLockProjectSameFileThroughSymlink(t *testing.T) {
	t.Parallel()
	fstree.Require(t, fstree.CapSymlink)

	realDir := t.TempDir()
	link := filepath.Join(t.TempDir(), "project-link")
	fstree.MustSymlink(t, realDir, link)
	cacheDir := t.TempDir()

	var paths []types.FilesystemPath
	for _, dir := range []string{realDir, link} {
		r, err := newResolverWithFetcher(types.FilesystemPath(dir), types.FilesystemPath(cacheDir), &fakeModuleFetcher{})
		if err != nil {
			t.Fatalf("newResolverWithFetcher(%s) error = %v", dir, err)
		}
		locker := &recordingLocker{inner: modulelock.NewFileLocker()}
		unlock, err := withRecordingLocker(r, locker).LockProject(t.Context())
		if err != nil {
			t.Fatalf("LockProject(%s) error = %v", dir, err)
		}
		if err := unlock(); err != nil {
			t.Fatalf("unlock(%s) error = %v", dir, err)
		}
		_, recorded := locker.recorded()
		paths = append(paths, recorded[0])
	}
	if paths[0] != paths[1] {
		t.Fatalf("lock files = %s and %s, want one file for both spellings", paths[0], paths[1])
	}
}
