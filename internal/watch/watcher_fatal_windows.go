// SPDX-License-Identifier: MPL-2.0

//go:build windows

package watch

import (
	"errors"
	"syscall"
)

// Windows system error codes from the Win32 API. Defined as package-level
// constants for clarity about which specific codes indicate a broken watcher.
const (
	// ERROR_TOO_MANY_OPEN_FILES (4): per-process handle limit exceeded.
	// Analogous to EMFILE on Unix.
	errnoTooManyOpenFiles = syscall.Errno(4)
	// ERROR_INVALID_HANDLE (6): the directory handle is no longer valid,
	// typically because the watched directory was deleted or unmounted.
	errnoInvalidHandle = syscall.Errno(6)
	// ERROR_NOT_ENOUGH_MEMORY (8): insufficient memory to allocate the
	// ReadDirectoryChangesW notification buffer.
	errnoNotEnoughMemory = syscall.Errno(8)
)

// isFatalFsnotifyError classifies fsnotify errors that indicate the watcher
// is fundamentally broken and cannot recover. On Windows, fsnotify uses
// ReadDirectoryChangesW which does not have inotify-style watch limits,
// but resource exhaustion and invalid handle errors still indicate an
// unrecoverable state:
//   - ERROR_TOO_MANY_OPEN_FILES: handle limit exceeded (analogous to EMFILE)
//   - ERROR_INVALID_HANDLE: watched directory deleted or handle invalidated
//   - ERROR_NOT_ENOUGH_MEMORY: cannot allocate notification buffer
func isFatalFsnotifyError(err error) bool {
	return errors.Is(err, errnoTooManyOpenFiles) ||
		errors.Is(err, errnoInvalidHandle) ||
		errors.Is(err, errnoNotEnoughMemory)
}
