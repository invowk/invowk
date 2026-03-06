// SPDX-License-Identifier: MPL-2.0

//go:build !linux

package testutil

import (
	"sync"
	"testing"
)

var containerSuiteMu sync.Mutex

func AcquireContainerSuiteLock(t testing.TB) func() {
	t.Helper()
	containerSuiteMu.Lock()
	return func() {
		containerSuiteMu.Unlock()
	}
}
