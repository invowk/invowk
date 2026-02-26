// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"testing"
)

func TestIsTransientError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		// Non-transient cases
		{name: "nil error", err: nil, want: false},
		{name: "context canceled", err: context.Canceled, want: false},
		{name: "context deadline", err: context.DeadlineExceeded, want: false},
		{name: "wrapped context canceled", err: fmt.Errorf("build failed: %w", context.Canceled), want: false},
		{name: "wrapped context deadline", err: fmt.Errorf("build failed: %w", context.DeadlineExceeded), want: false},
		{name: "generic error", err: errors.New("containerfile not found"), want: false},
		{name: "permission denied", err: errors.New("permission denied"), want: false},
		{name: "exit code 1", err: newExitError(t.Context(), 1), want: false},
		{name: "exit code 2", err: newExitError(t.Context(), 2), want: false},

		// Transient: exit code 125
		{name: "exit code 125", err: newExitError(t.Context(), 125), want: true},
		{name: "wrapped exit code 125", err: fmt.Errorf("build failed: %w", newExitError(t.Context(), 125)), want: true},

		// Transient: rootless Podman race conditions
		{name: "ping_group_range", err: errors.New("error reading /proc/sys/net/ipv4/ping_group_range"), want: true},
		{name: "OCI runtime error", err: errors.New("OCI runtime error: container_linux.go"), want: true},

		// Transient: network errors
		{name: "temporary failure resolving", err: errors.New("Temporary failure resolving 'deb.debian.org'"), want: true},
		{name: "could not resolve host", err: errors.New("Could not resolve host: registry-1.docker.io"), want: true},
		{name: "connection timed out", err: errors.New("connection timed out"), want: true},
		{name: "connection refused", err: errors.New("dial tcp: connection refused"), want: true},

		// Transient: storage errors
		{name: "overlay mount", err: errors.New("error creating overlay mount to /var/lib/containers"), want: true},
		{name: "mounting layer", err: errors.New("error mounting layer: invalid argument"), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := IsTransientError(tt.err)
			if got != tt.want {
				t.Errorf("IsTransientError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// newExitError creates an *exec.ExitError with the given exit code.
// It does this by running a command that exits with the specified code.
func newExitError(ctx context.Context, code int) *exec.ExitError {
	cmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("exit %d", code))
	err := cmd.Run()

	exitErr, ok := errors.AsType[*exec.ExitError](err)
	if !ok {
		return nil
	}
	return exitErr
}
