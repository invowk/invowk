// SPDX-License-Identifier: MPL-2.0

package commandadapters

import (
	"context"
	"sync"
	"testing"
)

// TestHostAccess verifies the SSH-backed host-access lifecycle: lazy start,
// idempotency, stop, restart, concurrency, and cancelled context.
// Subtests are sequential because the wish SSH library writes host keys to .ssh/
// in the working directory; parallel tests collide on the same key file.
func TestHostAccess(t *testing.T) { //nolint:paralleltest // subtests use t.Chdir and shared SSH host-key files.
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{name: "ensure starts server", run: func(t *testing.T) {
			t.Helper()
			host := newTestHostAccess(t)
			t.Cleanup(host.Stop)

			if err := host.Ensure(t.Context()); err != nil {
				t.Fatalf("Ensure() error = %v", err)
			}

			srv := host.SSHServer()
			if srv == nil {
				t.Fatal("SSHServer() = nil after Ensure()")
			}
			if !srv.IsRunning() {
				t.Fatal("server is not running after Ensure()")
			}
		}},
		{name: "ensure is idempotent", run: func(t *testing.T) {
			t.Helper()
			host := newTestHostAccess(t)
			t.Cleanup(host.Stop)

			if err := host.Ensure(t.Context()); err != nil {
				t.Fatalf("first Ensure() error = %v", err)
			}
			first := host.SSHServer()

			if err := host.Ensure(t.Context()); err != nil {
				t.Fatalf("second Ensure() error = %v", err)
			}
			second := host.SSHServer()

			if first != second {
				t.Fatal("second Ensure() created a different server; expected reuse")
			}
		}},
		{name: "stop shuts down server", run: func(t *testing.T) {
			t.Helper()
			host := newTestHostAccess(t)

			if err := host.Ensure(t.Context()); err != nil {
				t.Fatalf("Ensure() error = %v", err)
			}

			host.Stop()

			if host.SSHServer() != nil {
				t.Fatal("SSHServer() != nil after Stop()")
			}
		}},
		{name: "stop without start is safe", run: func(t *testing.T) {
			t.Helper()
			host := newTestHostAccess(t)
			host.Stop()

			if host.SSHServer() != nil {
				t.Fatal("SSHServer() != nil on never-started host access")
			}
		}},
		{name: "ensure after stop creates fresh server", run: func(t *testing.T) {
			t.Helper()
			host := newTestHostAccess(t)
			t.Cleanup(host.Stop)

			if err := host.Ensure(t.Context()); err != nil {
				t.Fatalf("first Ensure() error = %v", err)
			}
			first := host.SSHServer()

			host.Stop()

			if err := host.Ensure(t.Context()); err != nil {
				t.Fatalf("second Ensure() error = %v", err)
			}
			second := host.SSHServer()

			if second == nil {
				t.Fatal("SSHServer() = nil after re-Ensure()")
			}
			if first == second {
				t.Fatal("re-Ensure() returned same server pointer; expected fresh instance")
			}
		}},
		{name: "concurrent ensure starts exactly one server", run: func(t *testing.T) {
			t.Helper()
			host := newTestHostAccess(t)
			t.Cleanup(host.Stop)

			const goroutines = 5
			errs := make([]error, goroutines)

			var wg sync.WaitGroup
			wg.Add(goroutines)
			for i := range goroutines {
				go func() {
					defer wg.Done()
					errs[i] = host.Ensure(t.Context())
				}()
			}
			wg.Wait()

			for i, err := range errs {
				if err != nil {
					t.Fatalf("goroutine %d: Ensure() error = %v", i, err)
				}
			}

			srv := host.SSHServer()
			if srv == nil {
				t.Fatal("SSHServer() = nil after concurrent Ensure() calls")
			}
			if !srv.IsRunning() {
				t.Fatal("server is not running after concurrent Ensure() calls")
			}
		}},
		{name: "ensure with cancelled context returns error", run: func(t *testing.T) {
			t.Helper()
			host := newTestHostAccess(t)
			t.Cleanup(host.Stop)

			ctx, cancel := context.WithCancel(t.Context())
			cancel()

			err := host.Ensure(ctx)
			if err == nil {
				t.Fatal("Ensure() with cancelled context should return error")
			}

			if host.SSHServer() != nil {
				t.Fatal("SSHServer() != nil after failed Ensure()")
			}
		}},
	}
	//nolint:paralleltest // Cases use t.Chdir and shared SSH host-key files.
	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}

func newTestHostAccess(t testing.TB) *HostAccess {
	t.Helper()

	t.Chdir(t.TempDir())

	host, err := NewHostAccess()
	if err != nil {
		t.Fatalf("NewHostAccess() error = %v", err)
	}
	return host
}
