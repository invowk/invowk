// SPDX-License-Identifier: MPL-2.0

//go:build !linux

package runtime

import "errors"

// errFlockUnavailable is returned on non-Linux platforms where host-side flock
// cannot reach the Podman VM's filesystem. The caller falls back to sync.Mutex.
var errFlockUnavailable = errors.New("flock not available on this platform")

// acquireRunLock is a no-op on non-Linux platforms. On macOS/Windows, Podman runs
// inside a Linux VM (podman machine/WSL2) — a host-side flock doesn't reach the
// VM's filesystem. Returning an error causes the caller to fall back to the
// in-process sync.Mutex, which is the best available protection on these platforms.
func acquireRunLock() (*runLock, error) {
	return nil, errFlockUnavailable
}

// runLock is the non-Linux stub. Release is a no-op.
type runLock struct{}

// Release is a no-op on non-Linux platforms.
func (l *runLock) Release() {}
