// SPDX-License-Identifier: MPL-2.0

//go:build !linux

package testutil

import (
	"sync"
	"testing"
)

var containerSuiteMu sync.Mutex

// AcquireContainerSuiteLock is a no-op mutex-based fallback for non-Linux platforms
// where flock-based cross-process serialization is not available.
func AcquireContainerSuiteLock(t testing.TB) func() {
	t.Helper()
	containerSuiteMu.Lock()
	return func() {
		containerSuiteMu.Unlock()
	}
}
