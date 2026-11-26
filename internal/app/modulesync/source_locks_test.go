// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/modulelock"
	"github.com/invowk/invowk/pkg/types"
)

// sourceLockProject is a project whose invowkmod.cue requires a and whose
// lock is empty, with a's module source, for the source-lock tests.
func sourceLockProject(t *testing.T) (workDir, source string, before lockWindowFiles) {
	t.Helper()
	source = t.TempDir()
	writeSourceModule(t, source, "io.example.a")
	workDir = t.TempDir()
	before = buildLockWindowFiles(t, source, []GitURL{concurrentURLA}, []GitURL{})
	if err := writeLockWindowFiles(workDir, before); err != nil {
		t.Fatalf("write initial files: %v", err)
	}
	return workDir, source, before
}

func sha256Hex(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// TestLockOrderProjectThenSource checks design D4's order for a sync that
// fetches: project, source, source release, project release, and that the
// source lock file is named after sourceLockKey.
func TestLockOrderProjectThenSource(t *testing.T) {
	t.Parallel()

	workDir, source, _ := sourceLockProject(t)
	locker := &recordingLocker{}
	r := newLockWindowResolver(t, workDir, source, locker)

	if _, err := r.SyncModule(t.Context(), types.FilesystemPath(filepath.Join(workDir, "invowkmod.cue"))); err != nil {
		t.Fatalf("SyncModule() error = %v", err)
	}
	events, paths := locker.recorded()
	want := []string{"lock project", "lock source", "unlock source", "unlock project"}
	if !slices.Equal(events, want) {
		t.Fatalf("lock events = %v, want %v", events, want)
	}
	if got, wantName := filepath.Base(string(paths[1])), "source-"+sha256Hex(sourceLockKey(concurrentURLA))+".lock"; got != wantName {
		t.Fatalf("source lock file = %s, want %s", got, wantName)
	}
}

// TestLockOrderProjectUnderSourceRefused checks design D4's enforcement: a
// project lock requested while a source lock is held fails with
// errLockOrder without waiting, and so does a second source lock.
func TestLockOrderProjectUnderSourceRefused(t *testing.T) {
	t.Parallel()

	locker := &recordingLocker{}
	r := newLockWindowResolver(t, t.TempDir(), t.TempDir(), locker)
	unlockSource, err := r.lockSource(t.Context(), concurrentURLA)
	if err != nil {
		t.Fatalf("lockSource() error = %v", err)
	}
	if _, projectErr := r.LockProject(t.Context()); !errors.Is(projectErr, errLockOrder) {
		t.Fatalf("LockProject() under a source lock error = %v, want errLockOrder", projectErr)
	}
	if _, sourceErr := r.lockSource(t.Context(), concurrentURLB); !errors.Is(sourceErr, errLockOrder) {
		t.Fatalf("second lockSource() error = %v, want errLockOrder", sourceErr)
	}
	if unlockErr := unlockSource(); unlockErr != nil {
		t.Fatalf("source unlock error = %v", unlockErr)
	}
	unlockProject, err := r.LockProject(t.Context())
	if err != nil {
		t.Fatalf("LockProject() after the source release error = %v", err)
	}
	if err := unlockProject(); err != nil {
		t.Fatalf("project unlock error = %v", err)
	}
	if events, _ := locker.recorded(); !slices.Equal(events, []string{"lock source", "unlock source", "lock project", "unlock project"}) {
		t.Fatalf("lock events = %v, want only the granted locks", events)
	}
}

// sourceLockCommands are the commands that fetch a module source: add,
// sync and update, each on the sourceLockProject files.
func sourceLockCommands() map[string]func(ctx context.Context, r *Resolver, invowkmodPath types.FilesystemPath) (DeclarationRollback, error) {
	return map[string]func(ctx context.Context, r *Resolver, invowkmodPath types.FilesystemPath) (DeclarationRollback, error){
		"add": func(ctx context.Context, r *Resolver, invowkmodPath types.FilesystemPath) (DeclarationRollback, error) {
			result, err := r.AddModuleDependency(ctx, invowkmodPath, ModuleRef{GitURL: concurrentURLB, Version: "^1.0.0"})
			return result.Declaration().Rollback(), err
		},
		"sync": func(ctx context.Context, r *Resolver, invowkmodPath types.FilesystemPath) (DeclarationRollback, error) {
			_, err := r.SyncModule(ctx, invowkmodPath)
			return DeclarationRollbackNone, err
		},
		"update": func(ctx context.Context, r *Resolver, _ types.FilesystemPath) (DeclarationRollback, error) {
			_, err := r.UpdateModule(ctx, "")
			return DeclarationRollbackNone, err
		},
	}
}

func runSourceLockWait(t *testing.T, waitErr func(path types.FilesystemPath) error, check func(t *testing.T, err error, path types.FilesystemPath)) {
	t.Helper()
	for name, run := range sourceLockCommands() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			workDir, source, before := sourceLockProject(t)
			if name == "update" {
				before = buildLockWindowFiles(t, source, []GitURL{concurrentURLA}, []GitURL{concurrentURLA})
				if err := writeLockWindowFiles(workDir, before); err != nil {
					t.Fatalf("write initial files: %v", err)
				}
			}
			var sourcePath types.FilesystemPath
			locker := &recordingLocker{lockErr: func(path types.FilesystemPath) error {
				if lockKindOf(path) != "source" {
					return nil
				}
				sourcePath = path
				return waitErr(path)
			}}
			r := newLockWindowResolver(t, workDir, source, locker)

			rollback, err := run(t.Context(), r, types.FilesystemPath(filepath.Join(workDir, "invowkmod.cue")))
			if err == nil {
				t.Fatalf("%s error = nil, want the source lock wait's error", name)
			}
			check(t, err, sourcePath)
			if rollback != DeclarationRollbackNone {
				t.Fatalf("%s rollback = %s, want none", name, rollback)
			}
			assertLockWindowFiles(t, "files after a failed source lock wait", readLockWindowFiles(t, workDir), before)
		})
	}
}

// TestSourceLockWaitCancelled checks design D4: a cancelled source-lock wait
// makes add, sync and update fail with an error wrapping the cancellation,
// both files byte-identical, and add's rollback outcome none.
func TestSourceLockWaitCancelled(t *testing.T) {
	t.Parallel()
	runSourceLockWait(t,
		func(path types.FilesystemPath) error {
			return fmt.Errorf("waiting for lock file %s: %w", path, context.Canceled)
		},
		func(t *testing.T, err error, _ types.FilesystemPath) {
			t.Helper()
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want it to wrap context.Canceled", err)
			}
		})
}

// TestSourceLockWaitTimedOut checks design D4: a timed-out source-lock wait
// makes add, sync and update fail with an error from which errors.As
// recovers the *modulelock.TimeoutError, naming the source lock file.
func TestSourceLockWaitTimedOut(t *testing.T) {
	t.Parallel()
	runSourceLockWait(t,
		func(path types.FilesystemPath) error {
			return &modulelock.TimeoutError{Path: path, Timeout: time.Second}
		},
		func(t *testing.T, err error, path types.FilesystemPath) {
			t.Helper()
			if _, ok := errors.AsType[*modulelock.TimeoutError](err); !ok {
				t.Fatalf("error = %v, want a *modulelock.TimeoutError", err)
			}
			if path == "" || !strings.Contains(err.Error(), string(path)) || lockKindOf(path) != "source" {
				t.Fatalf("error = %v, want it to name the source lock file %s", err, path)
			}
		})
}
