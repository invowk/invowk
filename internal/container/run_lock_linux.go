// SPDX-License-Identifier: MPL-2.0

//go:build linux

package container

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/invowk/invowk/pkg/types"
	"golang.org/x/sys/unix"
)

// lockFileName is the well-known lock file name shared by all invowk processes.
// The zero-byte lock file is harmless if orphaned; the kernel releases the
// flock automatically when the fd is closed, including on process crash.
const lockFileName = "invowk-podman.lock"

// errFlockUnavailable is defined for cross-platform compatibility with
// run_lock_other.go. On Linux, acquireRunLock never returns this error; it is
// used by WithRunLock to distinguish expected non-Linux fallback from
// unexpected Linux lock acquisition failures.
var errFlockUnavailable = errors.New("flock not available on this platform")

// runLock holds a blocking exclusive flock on a well-known file path, providing
// cross-process serialization of Podman run calls. This prevents the rootless
// Podman ping_group_range race between concurrent invowk processes.
//
// The lock file lives in $XDG_RUNTIME_DIR with a fallback to os.TempDir when
// the env var is unset.
type runLock struct {
	file *os.File
}

// acquireRunLock opens or creates the lock file at the default platform path
// and acquires a blocking exclusive flock.
func acquireRunLock() (*runLock, error) {
	return acquireRunLockAt(lockFilePath())
}

// acquireRunLockAt opens or creates the lock file at the given path and
// acquires a blocking exclusive flock. This variant enables isolated tests.
func acquireRunLockAt(lockPath types.FilesystemPath) (*runLock, error) {
	f, err := os.OpenFile(string(lockPath), os.O_CREATE|os.O_RDWR, 0o600)
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
// multiple times; subsequent calls are no-ops.
func (l *runLock) Release() {
	if l == nil || l.file == nil {
		return
	}
	if err := unix.Flock(int(l.file.Fd()), unix.LOCK_UN); err != nil {
		slog.Warn("flock unlock failed", "error", err)
	}
	if err := l.file.Close(); err != nil {
		slog.Warn("lock file close failed", "error", err)
	}
	l.file = nil
}

func lockFilePath() types.FilesystemPath {
	return lockFilePathWith(os.Getenv)
}

func lockFilePathWith(getenv func(string) string) types.FilesystemPath {
	dir := getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return types.FilesystemPath(filepath.Join(dir, lockFileName)) //goplint:ignore -- OS path composition returns the lock file path opened immediately by this package
}
