// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"errors"
	"os/exec"
	"strings"
)

// IsTransientError reports whether err is a transient container engine error
// that may succeed on retry. It covers transient failures from both container
// run and build operations, including network timeouts, rootless Podman race
// conditions, storage driver glitches, and generic engine errors (exit code 125).
//
// Context cancellation and deadline errors are explicitly non-transient because
// retrying a cancelled operation is never useful.
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	// Context errors are never transient — the caller explicitly stopped the operation.
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	// Exit code 125 is a generic container engine error (e.g., Podman/Docker
	// internal failure). These are often transient storage or cgroup issues.
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr.ExitCode() == 125 {
		return true
	}

	errStr := err.Error()

	// Rootless Podman race conditions and OCI runtime errors.
	if strings.Contains(errStr, "ping_group_range") ||
		strings.Contains(errStr, "OCI runtime error") {
		return true
	}

	// Network errors during image pull or package installation inside builds.
	if strings.Contains(errStr, "Temporary failure resolving") ||
		strings.Contains(errStr, "Could not resolve host") ||
		strings.Contains(errStr, "connection timed out") ||
		strings.Contains(errStr, "connection refused") {
		return true
	}

	// Storage driver errors (overlay mount races on rootless Podman).
	if strings.Contains(errStr, "error creating overlay mount") ||
		strings.Contains(errStr, "error mounting layer") {
		return true
	}

	return false
}
