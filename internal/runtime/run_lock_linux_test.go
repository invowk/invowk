// SPDX-License-Identifier: MPL-2.0

//go:build linux

package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestAcquireRunLock_CreatesFile(t *testing.T) {
	t.Parallel()

	lockPath := filepath.Join(t.TempDir(), "test.lock")
	lock, err := acquireRunLockAt(lockPath)
	if err != nil {
		t.Fatalf("acquireRunLockAt() error: %v", err)
	}
	defer lock.Release()

	if _, statErr := os.Stat(lockPath); statErr != nil {
		t.Errorf("lock file not found at %s: %v", lockPath, statErr)
	}
}

func TestAcquireRunLock_BlocksConcurrent(t *testing.T) {
	t.Parallel()

	lockPath := filepath.Join(t.TempDir(), "test.lock")
	lockA, err := acquireRunLockAt(lockPath)
	if err != nil {
		t.Fatalf("acquireRunLockAt A: %v", err)
	}

	// Track whether goroutine B has acquired the lock.
	var acquired atomic.Bool

	done := make(chan struct{})
	go func() {
		defer close(done)
		lockB, bErr := acquireRunLockAt(lockPath)
		if bErr != nil {
			t.Errorf("acquireRunLockAt B: %v", bErr)
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

	lockPath := filepath.Join(t.TempDir(), "test.lock")
	lock, err := acquireRunLockAt(lockPath)
	if err != nil {
		t.Fatalf("acquireRunLockAt() error: %v", err)
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

// TestAcquireRunLockAt_SerializedAccess verifies that two goroutines calling
// acquireRunLockAt on the same path get serialized access to a shared resource.
// Without the lock, concurrent increments would race; the lock ensures each
// goroutine sees the result of the previous increment.
func TestAcquireRunLockAt_SerializedAccess(t *testing.T) {
	t.Parallel()

	lockPath := filepath.Join(t.TempDir(), "test.lock")
	counterPath := filepath.Join(t.TempDir(), "counter")

	// Initialize counter file to "0".
	if err := os.WriteFile(counterPath, []byte("0"), 0o600); err != nil {
		t.Fatalf("failed to write initial counter: %v", err)
	}

	const numGoroutines = 5
	done := make(chan struct{}, numGoroutines)

	for range numGoroutines {
		go func() {
			defer func() { done <- struct{}{} }()

			lock, lockErr := acquireRunLockAt(lockPath)
			if lockErr != nil {
				t.Errorf("acquireRunLockAt() error: %v", lockErr)
				return
			}
			defer lock.Release()

			// Read current counter.
			data, readErr := os.ReadFile(counterPath)
			if readErr != nil {
				t.Errorf("read counter: %v", readErr)
				return
			}

			var n int
			if _, scanErr := fmt.Sscanf(string(data), "%d", &n); scanErr != nil {
				t.Errorf("parse counter %q: %v", string(data), scanErr)
				return
			}

			// Increment and write back (non-atomic without the lock).
			n++
			if writeErr := os.WriteFile(counterPath, fmt.Appendf(nil, "%d", n), 0o600); writeErr != nil {
				t.Errorf("write counter: %v", writeErr)
				return
			}
		}()
	}

	// Wait for all goroutines.
	for range numGoroutines {
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for goroutines")
		}
	}

	// Verify final counter value.
	data, err := os.ReadFile(counterPath)
	if err != nil {
		t.Fatalf("read final counter: %v", err)
	}

	var finalCount int
	if _, scanErr := fmt.Sscanf(string(data), "%d", &finalCount); scanErr != nil {
		t.Fatalf("parse final counter %q: %v", string(data), scanErr)
	}

	if finalCount != numGoroutines {
		t.Errorf("counter = %d, want %d (serialization failure)", finalCount, numGoroutines)
	}
}

func TestLockFilePath_FallbackToTempDir(t *testing.T) {
	t.Parallel()

	path := lockFilePathWith(func(string) string { return "" })
	expected := filepath.Join(os.TempDir(), lockFileName)
	if path != expected {
		t.Errorf("lockFilePathWith() = %q, want %q", path, expected)
	}
}

func TestLockFilePath_UsesXDGRuntimeDir(t *testing.T) {
	t.Parallel()

	customDir := t.TempDir()
	path := lockFilePathWith(func(key string) string {
		if key == "XDG_RUNTIME_DIR" {
			return customDir
		}
		return ""
	})
	expected := filepath.Join(customDir, lockFileName)
	if path != expected {
		t.Errorf("lockFilePathWith() = %q, want %q", path, expected)
	}
}
