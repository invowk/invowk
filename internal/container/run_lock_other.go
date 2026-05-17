// SPDX-License-Identifier: MPL-2.0

//go:build !linux

package container

import "errors"

// errFlockUnavailable is returned on non-Linux platforms where host-side flock
// cannot reach the Podman VM's filesystem. Callers fall back to sync.Mutex.
var errFlockUnavailable = errors.New("flock not available on this platform")

// acquireRunLock is a no-op on non-Linux platforms. On macOS and Windows,
// Podman runs inside a VM, so a host-side flock does not protect container
// engine state inside that VM.
func acquireRunLock() (*runLock, error) {
	return nil, errFlockUnavailable
}

// runLock is the non-Linux stub.
type runLock struct{}

// Release is a no-op on non-Linux platforms.
func (l *runLock) Release() {}
