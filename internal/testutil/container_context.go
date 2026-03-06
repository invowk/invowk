// SPDX-License-Identifier: MPL-2.0

package testutil

import (
	"context"
	"testing"
	"time"
)

// DefaultContainerTestTimeout is the per-test deadline for container
// integration tests. It prevents indefinite hangs when the container daemon
// becomes unresponsive during builds or runs. Generous enough for slow CI
// runners; tight enough to fail fast on real stalls.
const DefaultContainerTestTimeout = 5 * time.Minute

// ContainerTestContext returns a context derived from t.Context() with a
// deadline. The cancel function is registered via t.Cleanup so callers
// don't need to defer it manually.
//
// Every container integration test that calls Execute() or ExecuteCapture()
// must use this instead of bare t.Context() to bound subprocess lifetime.
//
//	ctx := testutil.ContainerTestContext(t, testutil.DefaultContainerTestTimeout)
//	execCtx := NewExecutionContext(ctx, cmd, inv)
//	result := rt.Execute(execCtx)
func ContainerTestContext(t testing.TB, timeout time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	t.Cleanup(cancel)
	return ctx
}
