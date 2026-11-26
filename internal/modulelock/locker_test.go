// SPDX-License-Identifier: MPL-2.0

package modulelock_test

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/modulelock"
	"github.com/invowk/invowk/pkg/types"
)

const helperLockEnv = "INVOWK_MODULELOCK_HELPER_LOCK"

// TestMain lets the cross-process test re-execute this binary as a helper
// that holds a lock until it is killed.
func TestMain(m *testing.M) {
	if path := os.Getenv(helperLockEnv); path != "" {
		os.Exit(holdLockForever(path))
	}
	os.Exit(m.Run())
}

func holdLockForever(path string) int {
	unlock, err := modulelock.NewFileLocker().Lock(context.Background(), types.FilesystemPath(path), nil)
	if err != nil {
		_, _ = os.Stderr.WriteString("helper lock: " + err.Error() + "\n")
		return 2
	}
	defer func() { _ = unlock() }()
	_, _ = os.Stdout.WriteString("locked\n")
	// Block until the parent kills the process or closes stdin.
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	return 0
}

func lockPath(t *testing.T) types.FilesystemPath {
	t.Helper()
	return types.FilesystemPath(filepath.Join(t.TempDir(), "project-test.lock"))
}

func mustLock(t *testing.T, l *modulelock.FileLocker, path types.FilesystemPath) func() error {
	t.Helper()
	unlock, err := l.Lock(t.Context(), path, func() { t.Errorf("onBusy called for a free lock %s", path) })
	if err != nil {
		t.Fatalf("Lock(%s) error = %v", path, err)
	}
	return unlock
}

func TestFileLockerFreeLock(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	unlock := mustLock(t, modulelock.NewFileLocker(), path)
	info, err := os.Stat(string(path))
	if err != nil {
		t.Fatalf("Stat(lock file) error = %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("lock file size = %d, want an empty file", info.Size())
	}
	if err := unlock(); err != nil {
		t.Fatalf("unlock() error = %v", err)
	}
	if err := unlock(); err != nil {
		t.Fatalf("second unlock() error = %v, want nil", err)
	}
	if _, err := os.Stat(string(path)); err != nil {
		t.Fatalf("lock file after unlock: %v, want it kept", err)
	}
}

func TestFileLockerBusyLockWaitsOnceAndAcquires(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	unlockFirst := mustLock(t, modulelock.NewFileLocker(), path)

	var busyCalls atomic.Int32
	busy := make(chan struct{})
	type outcome struct {
		unlock func() error
		err    error
	}
	done := make(chan outcome, 1)
	second := modulelock.NewFileLockerForTest(5*time.Millisecond, time.Minute)
	go func() {
		unlock, err := second.Lock(t.Context(), path, func() {
			if busyCalls.Add(1) == 1 {
				close(busy)
			}
		})
		done <- outcome{unlock, err}
	}()
	select {
	case <-busy:
	case o := <-done:
		t.Fatalf("second Lock returned while the lock was held: %v", o.err)
	case <-time.After(30 * time.Second):
		t.Fatal("second Lock never reported the busy lock")
	}
	if err := unlockFirst(); err != nil {
		t.Fatalf("first unlock() error = %v", err)
	}
	o := <-done
	if o.err != nil {
		t.Fatalf("second Lock() error = %v", o.err)
	}
	if got := busyCalls.Load(); got != 1 {
		t.Fatalf("onBusy calls = %d, want exactly 1", got)
	}
	if err := o.unlock(); err != nil {
		t.Fatalf("second unlock() error = %v", err)
	}
}

func TestFileLockerTimeoutNamesPath(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	unlock := mustLock(t, modulelock.NewFileLocker(), path)
	defer func() { _ = unlock() }()

	_, err := modulelock.NewFileLockerForTest(5*time.Millisecond, 50*time.Millisecond).Lock(t.Context(), path, nil)
	var timeoutErr *modulelock.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("Lock() error = %v, want *TimeoutError", err)
	}
	if timeoutErr.Path != path {
		t.Fatalf("TimeoutError.Path = %s, want %s", timeoutErr.Path, path)
	}
	for _, want := range []string{"timed out after 50ms waiting for lock file " + string(path), "another invowk process holds it"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Lock() error = %q, want it to contain %q", err, want)
		}
	}
}

func TestFileLockerCancelledWait(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	unlock := mustLock(t, modulelock.NewFileLocker(), path)
	defer func() { _ = unlock() }()

	ctx, cancel := context.WithCancel(t.Context())
	_, err := modulelock.NewFileLockerForTest(5*time.Millisecond, time.Minute).Lock(ctx, path, cancel)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Lock() error = %v, want context.Canceled", err)
	}
	if !strings.Contains(err.Error(), "waiting for lock file "+string(path)) {
		t.Fatalf("Lock() error = %q, want it to name the lock file", err)
	}
}

func TestFileLockerRefusesSameInstanceRelock(t *testing.T) {
	t.Parallel()
	path := lockPath(t)
	locker := modulelock.NewFileLocker()
	unlock := mustLock(t, locker, path)

	if _, err := locker.Lock(t.Context(), path, nil); !errors.Is(err, modulelock.ErrLockHeld) {
		t.Fatalf("second Lock() on the same FileLocker error = %v, want ErrLockHeld", err)
	}
	if err := unlock(); err != nil {
		t.Fatalf("unlock() error = %v", err)
	}
	relock := mustLock(t, locker, path)
	if err := relock(); err != nil {
		t.Fatalf("unlock() after relock error = %v", err)
	}
}

func TestFileLockerOpenFailureNamesPath(t *testing.T) {
	t.Parallel()
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	path := types.FilesystemPath(filepath.Join(blocker, "project.lock"))
	_, err := modulelock.NewFileLocker().Lock(t.Context(), path, nil)
	if err == nil || !strings.Contains(err.Error(), string(path)) {
		t.Fatalf("Lock() under a regular file error = %v, want an error naming %s", err, path)
	}
}

// TestFileLockerCrossProcessReleasedOnDeath runs a helper process (this test
// binary) that holds the lock, checks that this process waits for it, and
// kills the helper: the OS releases the lock and the wait ends.
func TestFileLockerCrossProcessReleasedOnDeath(t *testing.T) {
	t.Parallel()
	path := lockPath(t)

	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), helperLockEnv+"="+string(path))
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("StdinPipe() error = %v", err)
	}
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("StdoutPipe() error = %v", err)
	}
	cmd.Stderr = os.Stderr
	if startErr := cmd.Start(); startErr != nil {
		t.Fatalf("start helper: %v", startErr)
	}
	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || line != "locked\n" {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("helper said %q, error %v; want \"locked\"", line, err)
	}

	busy := make(chan struct{})
	done := make(chan error, 1)
	var unlock func() error
	go func() {
		var lockErr error
		unlock, lockErr = modulelock.NewFileLockerForTest(5*time.Millisecond, time.Minute).Lock(t.Context(), path, func() { close(busy) })
		done <- lockErr
	}()
	select {
	case <-busy:
	case err := <-done:
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("Lock returned while the helper held the lock: %v", err)
	case <-time.After(30 * time.Second):
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatal("Lock never reported the helper's lock as busy")
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("kill helper: %v", err)
	}
	_ = cmd.Wait()
	if err := <-done; err != nil {
		t.Fatalf("Lock() after the helper died error = %v", err)
	}
	if err := unlock(); err != nil {
		t.Fatalf("unlock() error = %v", err)
	}
}
