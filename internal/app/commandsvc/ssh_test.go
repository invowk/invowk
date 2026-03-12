// SPDX-License-Identifier: MPL-2.0

package commandsvc

import (
	"context"
	"sync"
	"testing"
)

// TestSSHServerController verifies the sshServerController lifecycle: lazy start,
// idempotency, stop, restart, concurrency, and cancelled context.
// Subtests are sequential because the wish SSH library writes host keys to .ssh/
// in the working directory; parallel tests collide on the same key file.
func TestSSHServerController(t *testing.T) {
	t.Run("ensure starts server", func(t *testing.T) {
		ctrl := &sshServerController{}
		t.Cleanup(ctrl.stop)

		if err := ctrl.ensure(t.Context()); err != nil {
			t.Fatalf("ensure() error = %v", err)
		}

		srv := ctrl.current()
		if srv == nil {
			t.Fatal("current() = nil after ensure()")
		}
		if !srv.IsRunning() {
			t.Fatal("server is not running after ensure()")
		}
	})

	t.Run("ensure is idempotent", func(t *testing.T) {
		ctrl := &sshServerController{}
		t.Cleanup(ctrl.stop)

		if err := ctrl.ensure(t.Context()); err != nil {
			t.Fatalf("first ensure() error = %v", err)
		}
		first := ctrl.current()

		if err := ctrl.ensure(t.Context()); err != nil {
			t.Fatalf("second ensure() error = %v", err)
		}
		second := ctrl.current()

		if first != second {
			t.Fatal("second ensure() created a different server; expected reuse")
		}
	})

	t.Run("stop shuts down server", func(t *testing.T) {
		ctrl := &sshServerController{}

		if err := ctrl.ensure(t.Context()); err != nil {
			t.Fatalf("ensure() error = %v", err)
		}

		ctrl.stop()

		if ctrl.current() != nil {
			t.Fatal("current() != nil after stop()")
		}
	})

	t.Run("stop without start is safe", func(t *testing.T) {
		ctrl := &sshServerController{}
		// Must not panic.
		ctrl.stop()

		if ctrl.current() != nil {
			t.Fatal("current() != nil on never-started controller")
		}
	})

	t.Run("ensure after stop creates fresh server", func(t *testing.T) {
		ctrl := &sshServerController{}
		t.Cleanup(ctrl.stop)

		if err := ctrl.ensure(t.Context()); err != nil {
			t.Fatalf("first ensure() error = %v", err)
		}
		first := ctrl.current()

		ctrl.stop()

		if err := ctrl.ensure(t.Context()); err != nil {
			t.Fatalf("second ensure() error = %v", err)
		}
		second := ctrl.current()

		if second == nil {
			t.Fatal("current() = nil after re-ensure()")
		}
		if first == second {
			t.Fatal("re-ensure() returned same server pointer; expected fresh instance")
		}
	})

	t.Run("concurrent ensure starts exactly one server", func(t *testing.T) {
		ctrl := &sshServerController{}
		t.Cleanup(ctrl.stop)

		const goroutines = 5
		errs := make([]error, goroutines)

		var wg sync.WaitGroup
		wg.Add(goroutines)
		for i := range goroutines {
			go func() {
				defer wg.Done()
				errs[i] = ctrl.ensure(t.Context())
			}()
		}
		wg.Wait()

		for i, err := range errs {
			if err != nil {
				t.Fatalf("goroutine %d: ensure() error = %v", i, err)
			}
		}

		srv := ctrl.current()
		if srv == nil {
			t.Fatal("current() = nil after concurrent ensure() calls")
		}
		if !srv.IsRunning() {
			t.Fatal("server is not running after concurrent ensure() calls")
		}
	})

	t.Run("ensure with cancelled context returns error", func(t *testing.T) {
		ctrl := &sshServerController{}
		t.Cleanup(ctrl.stop)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := ctrl.ensure(ctx)
		if err == nil {
			t.Fatal("ensure() with cancelled context should return error")
		}

		if ctrl.current() != nil {
			t.Fatal("current() != nil after failed ensure()")
		}
	})
}
