// SPDX-License-Identifier: MPL-2.0

//go:build linux

package runtime

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// lockFileName is the well-known lock file name shared by all invowk processes.
// The zero-byte lock file is harmless if orphaned — the kernel releases the
// flock automatically when the fd is closed (including on process crash).
const lockFileName = "invowk-podman.lock"

// errFlockUnavailable is defined for cross-platform compatibility with
// run_lock_other.go. On Linux, acquireRunLock() never returns this error —
// it is only used by container_exec.go to distinguish expected (non-Linux)
// from unexpected (Linux) lock acquisition failures.
var errFlockUnavailable = errors.New("flock not available on this platform")

// runLock holds a blocking exclusive flock on a well-known file path, providing
// cross-process serialization of container run calls. This prevents the rootless
// Podman ping_group_range race between concurrent invowk processes (testscript
// tests, parallel terminal invocations).
//
// The lock file lives in $XDG_RUNTIME_DIR (per-user tmpfs, fast, auto-cleaned)
// with a fallback to os.TempDir() when the env var is unset.
type runLock struct {
	file *os.File
}

// acquireRunLock opens (or creates) the lock file and acquires a blocking
// exclusive flock. The call blocks until the lock is available.
func acquireRunLock() (*runLock, error) {
	lockPath := lockFilePath()

	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", lockPath, err)
	}

	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		f.Close()
		return nil, fmt.Errorf("flock %s: %w", lockPath, err)
	}

	return &runLock{file: f}, nil
}

// Release unlocks the flock and closes the file descriptor. It is safe to call
// multiple times — subsequent calls are no-ops.
func (l *runLock) Release() {
	if l == nil || l.file == nil {
		return
	}
	// LOCK_UN before Close for explicitness; Close also releases the flock.
	if err := unix.Flock(int(l.file.Fd()), unix.LOCK_UN); err != nil {
		slog.Debug("flock unlock failed", "error", err)
	}
	if err := l.file.Close(); err != nil {
		slog.Debug("lock file close failed", "error", err)
	}
	l.file = nil
}

// lockFilePath returns the path for the cross-process lock file.
// Prefers $XDG_RUNTIME_DIR (per-user tmpfs), falls back to os.TempDir().
func lockFilePath() string {
	return lockFilePathWith(os.Getenv)
}

// lockFilePathWith returns the lock file path using the provided getenv function.
// This enables testing without mutating process-global environment state.
func lockFilePathWith(getenv func(string) string) string {
	dir := getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, lockFileName)
}
