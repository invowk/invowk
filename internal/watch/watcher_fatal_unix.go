// SPDX-License-Identifier: MPL-2.0

//go:build !windows

package watch

import (
	"errors"
	"syscall"
)

// isFatalFsnotifyError classifies fsnotify errors that indicate the watcher
// is fundamentally broken and cannot recover. On Linux, these correspond to
// inotify resource exhaustion errors:
//   - ENOSPC: inotify watch limit exceeded (fs.inotify.max_user_watches)
//   - EMFILE: per-process file descriptor limit exceeded
//   - ENFILE: system-wide file descriptor limit exceeded
func isFatalFsnotifyError(err error) bool {
	return errors.Is(err, syscall.ENOSPC) ||
		errors.Is(err, syscall.EMFILE) ||
		errors.Is(err, syscall.ENFILE)
}
