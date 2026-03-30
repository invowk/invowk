// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"os"
	"runtime"
	"strconv"
	"sync"
	"testing"
)

// ContainerSemaphore returns a process-wide buffered channel that limits concurrent
// container operations in tests. Acquire a slot by sending, release by receiving:
//
//	sem := testutil.ContainerSemaphore()
//	sem <- struct{}{}
//	defer func() { <-sem }()
//
// The capacity is determined by INVOWK_TEST_CONTAINER_PARALLEL (if set) or
// min(GOMAXPROCS, 2). Capping at 2 prevents Podman resource exhaustion on
// constrained CI runners where too many concurrent container operations cause
// indefinite hangs rather than retryable errors.
var ContainerSemaphore = sync.OnceValue(func() chan struct{} {
	n := containerParallelism()
	return make(chan struct{}, n)
})

// AcquireContainerSemaphore acquires a slot from the process-wide container
// semaphore and registers t.Cleanup to release it when the test finishes.
// Place this call after t.Parallel() and testing.Short() skip, before container ops.
func AcquireContainerSemaphore(t testing.TB) {
	t.Helper()
	sem := ContainerSemaphore()
	sem <- struct{}{}
	t.Cleanup(func() { <-sem })
}

// containerParallelism returns the number of concurrent container operations allowed.
// It checks INVOWK_TEST_CONTAINER_PARALLEL first, then falls back to min(GOMAXPROCS, 2).
func containerParallelism() int {
	if v := os.Getenv("INVOWK_TEST_CONTAINER_PARALLEL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return min(runtime.GOMAXPROCS(0), 2)
}
