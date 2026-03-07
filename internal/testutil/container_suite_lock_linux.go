// SPDX-License-Identifier: MPL-2.0

//go:build linux

package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

const containerSuiteLockFileName = "invowk-container-suite.lock"

type containerSuiteLock struct {
	file *os.File
}

// AcquireContainerSuiteLock serializes real-container test suites across test
// processes on Linux using a blocking exclusive flock on a well-known file.
func AcquireContainerSuiteLock(t testing.TB) func() {
	t.Helper()

	lockPath := containerSuiteLockFilePath()
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("open container suite lock %s: %v", lockPath, err)
	}

	if err := unix.Flock(int(lockFile.Fd()), unix.LOCK_EX); err != nil {
		_ = lockFile.Close()
		t.Fatalf("flock container suite lock %s: %v", lockPath, err)
	}

	lock := &containerSuiteLock{file: lockFile}
	return lock.Release
}

func (l *containerSuiteLock) Release() {
	if l == nil || l.file == nil {
		return
	}
	unlockErr := unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
	_ = unlockErr
	_ = l.file.Close()
	l.file = nil
}

func containerSuiteLockFilePath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		dir = os.TempDir()
	}
	return filepath.Join(dir, containerSuiteLockFileName)
}
