// SPDX-License-Identifier: MPL-2.0

package modulelock

import "time"

// TrimFinalPathPrefix exposes trimFinalPathPrefix to the table test.
var TrimFinalPathPrefix = trimFinalPathPrefix

// NewFileLockerForTest returns a FileLocker with a shortened poll interval
// and timeout.
func NewFileLockerForTest(poll, timeout time.Duration) *FileLocker {
	l := NewFileLocker()
	l.pollInterval = poll
	l.timeout = timeout
	return l
}
