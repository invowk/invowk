// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"
)

const (
	prepareCommandBlockedDuration = 50 * time.Millisecond
	prepareCommandReleaseTimeout  = time.Second
)

func TestBaseCLIEngine_RunSerializationDecision(t *testing.T) {
	originalAcquire := acquireContainerLock
	var lockAttempts atomic.Int32
	acquireContainerLock = func() (*runLock, error) {
		lockAttempts.Add(1)
		return nil, errFlockUnavailable
	}
	t.Cleanup(func() {
		acquireContainerLock = originalAcquire
	})

	tests := []struct {
		name                 string
		engineName           string
		sysctlOverrideActive bool
		wantLockAttempts     int32
	}{
		{
			name:             "docker engine skips serialization",
			engineName:       string(EngineTypeDocker),
			wantLockAttempts: 0,
		},
		{
			name:                 "podman with override active skips serialization",
			engineName:           string(EngineTypePodman),
			sysctlOverrideActive: true,
			wantLockAttempts:     0,
		},
		{
			name:             "podman with override inactive acquires serialization",
			engineName:       string(EngineTypePodman),
			wantLockAttempts: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lockAttempts.Store(0)
			recorder := NewMockCommandRecorder()
			engine := NewBaseCLIEngine(
				HostFilesystemPath("/usr/bin/"+tt.engineName),
				WithName(tt.engineName),
				WithSysctlOverrideActive(tt.sysctlOverrideActive),
				WithExecCommand(recorder.ContextCommandFunc(t)),
			)

			result, err := engine.Run(t.Context(), RunOptions{
				Image:   ImageTag("debian:stable-slim"),
				Command: []string{"echo", "hello"},
			})
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if result.ExitCode != 0 {
				t.Fatalf("Run() exit code = %d, want 0", result.ExitCode)
			}
			if got := lockAttempts.Load(); got != tt.wantLockAttempts {
				t.Fatalf("lock attempts = %d, want %d", got, tt.wantLockAttempts)
			}
		})
	}
}

func TestBaseCLIEngine_FallbackSerializerSharedAcrossEngines(t *testing.T) {
	originalAcquire := acquireContainerLock
	acquireContainerLock = func() (*runLock, error) {
		return nil, errFlockUnavailable
	}
	t.Cleanup(func() {
		acquireContainerLock = originalAcquire
	})

	start := make(chan struct{})
	var active atomic.Int32
	var maxActive atomic.Int32
	execFn := func(ctx context.Context, _ string, _ ...string) *exec.Cmd {
		<-start
		current := active.Add(1)
		updateMaxInt32(&maxActive, current)
		time.Sleep(50 * time.Millisecond)
		active.Add(-1)
		return exec.CommandContext(ctx, "true")
	}

	newEngine := func() *BaseCLIEngine {
		return NewBaseCLIEngine(
			"/usr/bin/podman",
			WithName(string(EngineTypePodman)),
			WithExecCommand(execFn),
		)
	}

	errCh := make(chan error, 2)
	run := func(engine *BaseCLIEngine) {
		_, runErr := engine.Run(t.Context(), RunOptions{Image: ImageTag("debian:stable-slim")})
		errCh <- runErr
	}
	go run(newEngine())
	go run(newEngine())
	close(start)

	for range 2 {
		if err := <-errCh; err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	}
	if got := maxActive.Load(); got != 1 {
		t.Fatalf("max concurrent runs = %d, want 1", got)
	}
}

func TestBaseCLIEngine_PrepareRunCommandHoldsSerializationUntilCleanup(t *testing.T) {
	originalAcquire := acquireContainerLock
	var lockAttempts atomic.Int32
	acquireContainerLock = func() (*runLock, error) {
		lockAttempts.Add(1)
		return nil, errFlockUnavailable
	}
	t.Cleanup(func() {
		acquireContainerLock = originalAcquire
	})

	engine := NewBaseCLIEngine("/usr/bin/podman", WithName(string(EngineTypePodman)))
	_, firstCleanup, err := engine.PrepareRunCommand(t.Context(), RunOptions{Image: ImageTag("debian:stable-slim")})
	if err != nil {
		t.Fatalf("PrepareRunCommand() error = %v", err)
	}
	if firstCleanup == nil {
		t.Fatal("PrepareRunCommand() cleanup = nil, want serialization cleanup")
	}
	defer func() {
		if firstCleanup != nil {
			firstCleanup()
		}
	}()

	secondPrepared := make(chan struct{})
	secondDone := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		close(secondPrepared)
		_, secondCleanup, prepErr := engine.PrepareRunCommand(t.Context(), RunOptions{Image: ImageTag("debian:stable-slim")})
		if secondCleanup != nil {
			secondCleanup()
		}
		errCh <- prepErr
		close(secondDone)
	}()
	<-secondPrepared

	select {
	case <-secondDone:
		t.Fatal("second PrepareRunCommand returned before first cleanup released serialization")
	case <-time.After(prepareCommandBlockedDuration):
	}

	firstCleanup()
	firstCleanup = nil

	select {
	case <-secondDone:
	case <-time.After(prepareCommandReleaseTimeout):
		t.Fatal("second PrepareRunCommand did not return after first cleanup")
	}
	if err := <-errCh; err != nil {
		t.Fatalf("second PrepareRunCommand() error = %v", err)
	}
	if got := lockAttempts.Load(); got != 2 {
		t.Fatalf("lock attempts = %d, want 2", got)
	}
}

func TestBaseCLIEngine_PrepareRunCommandSkipsSerializationForDocker(t *testing.T) {
	originalAcquire := acquireContainerLock
	var lockAttempts atomic.Int32
	acquireContainerLock = func() (*runLock, error) {
		lockAttempts.Add(1)
		return nil, errFlockUnavailable
	}
	t.Cleanup(func() {
		acquireContainerLock = originalAcquire
	})

	engine := NewBaseCLIEngine("/usr/bin/docker", WithName(string(EngineTypeDocker)))
	_, cleanup, err := engine.PrepareRunCommand(t.Context(), RunOptions{Image: ImageTag("debian:stable-slim")})
	if err != nil {
		t.Fatalf("PrepareRunCommand() error = %v", err)
	}
	if cleanup != nil {
		t.Fatal("PrepareRunCommand() cleanup != nil for docker")
	}
	if got := lockAttempts.Load(); got != 0 {
		t.Fatalf("lock attempts = %d, want 0", got)
	}
}

func TestBaseCLIEngine_CoordinateLifecycleSerializationDecision(t *testing.T) {
	originalAcquire := acquireContainerLock
	var lockAttempts atomic.Int32
	acquireContainerLock = func() (*runLock, error) {
		lockAttempts.Add(1)
		return nil, errFlockUnavailable
	}
	t.Cleanup(func() {
		acquireContainerLock = originalAcquire
	})

	tests := []struct {
		name                 string
		engineName           string
		sysctlOverrideActive bool
		wantLockAttempts     int32
	}{
		{
			name:             "docker skips serialization",
			engineName:       string(EngineTypeDocker),
			wantLockAttempts: 0,
		},
		{
			name:                 "podman with override active skips serialization",
			engineName:           string(EngineTypePodman),
			sysctlOverrideActive: true,
			wantLockAttempts:     0,
		},
		{
			name:             "podman with override inactive serializes",
			engineName:       string(EngineTypePodman),
			wantLockAttempts: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lockAttempts.Store(0)
			engine := NewBaseCLIEngine(
				HostFilesystemPath("/usr/bin/"+tt.engineName),
				WithName(tt.engineName),
				WithSysctlOverrideActive(tt.sysctlOverrideActive),
			)

			var called atomic.Bool
			if err := engine.CoordinateLifecycle(func() error {
				called.Store(true)
				return nil
			}); err != nil {
				t.Fatalf("CoordinateLifecycle() error = %v", err)
			}
			if !called.Load() {
				t.Fatal("CoordinateLifecycle() did not call callback")
			}
			if got := lockAttempts.Load(); got != tt.wantLockAttempts {
				t.Fatalf("lock attempts = %d, want %d", got, tt.wantLockAttempts)
			}
		})
	}
}

func updateMaxInt32(maxCounter *atomic.Int32, candidate int32) {
	for {
		observed := maxCounter.Load()
		if candidate <= observed || maxCounter.CompareAndSwap(observed, candidate) {
			return
		}
	}
}

func TestDockerEngine_DoesNotImplementSysctlChecker(t *testing.T) {
	t.Parallel()

	var engine Engine = &DockerEngine{BaseCLIEngine: NewBaseCLIEngine("/usr/bin/docker", WithName(string(EngineTypeDocker)))}
	if _, ok := engine.(SysctlOverrideChecker); ok {
		t.Fatal("DockerEngine must not implement SysctlOverrideChecker")
	}
}

func TestPodmanEngine_ImplementsSysctlChecker(t *testing.T) {
	t.Parallel()

	var engine Engine = &PodmanEngine{BaseCLIEngine: NewBaseCLIEngine(
		"/usr/bin/podman",
		WithName(string(EngineTypePodman)),
		WithSysctlOverrideActive(true),
	)}
	checker, ok := engine.(SysctlOverrideChecker)
	if !ok {
		t.Fatal("PodmanEngine must implement SysctlOverrideChecker")
	}
	if !checker.SysctlOverrideActive() {
		t.Fatal("SysctlOverrideActive() = false, want true")
	}
}
