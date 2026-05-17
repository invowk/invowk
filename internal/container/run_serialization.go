// SPDX-License-Identifier: MPL-2.0

package container

import (
	"errors"
	"log/slog"
	"sync"
)

var (
	runFallbackMu        sync.Mutex
	acquireContainerLock = acquireRunLock
)

// WithRunLock serializes container operations that need the shared Podman run
// gate. Linux uses flock for cross-process protection; other platforms fall
// back to a process-wide mutex.
func WithRunLock(fn func() error) error {
	cleanup := acquireRunLockCleanup()
	defer cleanup()
	return fn()
}

func acquireRunLockCleanup() func() {
	lock, lockErr := acquireContainerLock()
	if lockErr != nil {
		if errors.Is(lockErr, errFlockUnavailable) {
			slog.Debug("flock unavailable, falling back to in-process mutex", "error", lockErr)
		} else {
			slog.Warn("flock acquisition failed, falling back to in-process mutex", "error", lockErr)
		}
		runFallbackMu.Lock()
		return runFallbackMu.Unlock
	}
	return lock.Release
}

func needsPodmanRunSerialization(engineName EngineType, sysctlOverrideActive bool) bool {
	return engineName == EngineTypePodman && !sysctlOverrideActive
}

func (e *BaseCLIEngine) withRunSerialization(fn func() (*RunResult, error)) (*RunResult, error) {
	if !needsPodmanRunSerialization(EngineType(e.name), e.sysctlOverrideActive) { //goplint:ignore -- BaseCLIEngine names are initialized from EngineType constants
		return fn()
	}

	var result *RunResult
	err := WithRunLock(func() error {
		var runErr error
		result, runErr = fn()
		return runErr
	})
	return result, err
}

func (e *BaseCLIEngine) runSerializationCleanup() func() {
	if !needsPodmanRunSerialization(EngineType(e.name), e.sysctlOverrideActive) { //goplint:ignore -- BaseCLIEngine names are initialized from EngineType constants
		return nil
	}
	return acquireRunLockCleanup()
}
