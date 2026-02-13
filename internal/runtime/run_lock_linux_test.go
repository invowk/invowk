// SPDX-License-Identifier: MPL-2.0

//go:build linux

package runtime

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestAcquireRunLock_CreatesFile(t *testing.T) {
	t.Parallel()

	lock, err := acquireRunLock()
	if err != nil {
		t.Fatalf("acquireRunLock() error: %v", err)
	}
	defer lock.Release()

	path := lockFilePath()
	if _, statErr := os.Stat(path); statErr != nil {
		t.Errorf("lock file not found at %s: %v", path, statErr)
	}
}

func TestAcquireRunLock_BlocksConcurrent(t *testing.T) {
	t.Parallel()

	lockA, err := acquireRunLock()
	if err != nil {
		t.Fatalf("acquireRunLock A: %v", err)
	}

	// Track whether goroutine B has acquired the lock.
	var acquired atomic.Bool

	done := make(chan struct{})
	go func() {
		defer close(done)
		lockB, bErr := acquireRunLock()
		if bErr != nil {
			t.Errorf("acquireRunLock B: %v", bErr)
			return
		}
		acquired.Store(true)
		lockB.Release()
	}()

	// Give goroutine B time to attempt the lock. It should be blocked.
	time.Sleep(100 * time.Millisecond)
	if acquired.Load() {
		t.Fatal("goroutine B acquired the lock while A still held it")
	}

	// Release A — B should now acquire.
	lockA.Release()

	select {
	case <-done:
		if !acquired.Load() {
			t.Fatal("goroutine B never acquired the lock after A released")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for goroutine B to acquire the lock")
	}
}

func TestRunLock_Release_Idempotent(t *testing.T) {
	t.Parallel()

	lock, err := acquireRunLock()
	if err != nil {
		t.Fatalf("acquireRunLock() error: %v", err)
	}

	// Double-release must not panic.
	lock.Release()
	lock.Release()
}

func TestRunLock_Release_NilReceiver(t *testing.T) {
	t.Parallel()

	// Nil receiver must not panic (defensive for error paths).
	var lock *runLock
	lock.Release()
}

func TestAcquireRunLock_FallbackToTempDir(t *testing.T) {
	// Cannot use t.Parallel() — t.Setenv modifies process-wide state.
	t.Setenv("XDG_RUNTIME_DIR", "")

	path := lockFilePath()
	expected := filepath.Join(os.TempDir(), lockFileName)
	if path != expected {
		t.Errorf("lockFilePath() = %q, want %q", path, expected)
	}

	lock, err := acquireRunLock()
	if err != nil {
		t.Fatalf("acquireRunLock() with empty XDG_RUNTIME_DIR: %v", err)
	}
	lock.Release()
}

func TestAcquireRunLock_UsesXDGRuntimeDir(t *testing.T) {
	// Cannot use t.Parallel() — t.Setenv modifies process-wide state.
	customDir := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", customDir)

	path := lockFilePath()
	expected := filepath.Join(customDir, lockFileName)
	if path != expected {
		t.Errorf("lockFilePath() = %q, want %q", path, expected)
	}

	lock, err := acquireRunLock()
	if err != nil {
		t.Fatalf("acquireRunLock() with custom XDG_RUNTIME_DIR: %v", err)
	}
	defer lock.Release()

	// Verify the lock file was created in the custom directory.
	if _, statErr := os.Stat(expected); statErr != nil {
		t.Errorf("lock file not found at %s: %v", expected, statErr)
	}
}
