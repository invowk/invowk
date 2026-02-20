// SPDX-License-Identifier: MPL-2.0

//go:build windows

package watch

// isFatalFsnotifyError classifies fsnotify errors that indicate the watcher
// is fundamentally broken and cannot recover. On Windows, fsnotify uses
// ReadDirectoryChangesW which does not have inotify-style resource limits,
// so no errors are classified as fatal.
func isFatalFsnotifyError(_ error) bool {
	return false
}
