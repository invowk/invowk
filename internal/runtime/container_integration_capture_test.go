// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// TestContainerRuntime_ExecuteCapture tests the ExecuteCapture method that captures stdout/stderr
func TestContainerRuntime_ExecuteCapture(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if we can run containers using our own engine detection
	engine, err := container.AutoDetectEngine()
	if err != nil {
		t.Skipf("skipping container integration tests: no container engine available: %v", err)
	}
	if !engine.Available() {
		t.Skip("skipping container integration tests: container engine not available")
	}

	t.Run("BasicCapture", func(t *testing.T) {
		t.Parallel()
		sem := testutil.ContainerSemaphore()
		sem <- struct{}{}
		defer func() { <-sem }()
		testContainerExecuteCaptureBasic(t)
	})
	t.Run("CaptureWithExitCode", func(t *testing.T) {
		t.Parallel()
		sem := testutil.ContainerSemaphore()
		sem <- struct{}{}
		defer func() { <-sem }()
		testContainerExecuteCaptureExitCode(t)
	})
	t.Run("CaptureStderr", func(t *testing.T) {
		t.Parallel()
		sem := testutil.ContainerSemaphore()
		sem <- struct{}{}
		defer func() { <-sem }()
		testContainerExecuteCaptureStderr(t)
	})
	t.Run("CaptureWithEnvVars", func(t *testing.T) {
		t.Parallel()
		sem := testutil.ContainerSemaphore()
		sem <- struct{}{}
		defer func() { <-sem }()
		testContainerExecuteCaptureEnvVars(t)
	})
}

// testContainerExecuteCaptureBasic tests basic output capture
func testContainerExecuteCaptureBasic(t *testing.T) {
	t.Helper()
	_, inv := setupTestInvowkfile(t)

	cmd := &invowkfile.Command{
		Name: "test-capture-basic",
		Implementations: []invowkfile.Implementation{
			{
				Script: "echo 'Hello from captured container'",
				Runtimes: []invowkfile.RuntimeConfig{
					{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"},
				},
				Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}},
			},
		},
	}

	rt := createContainerRuntime(t)
	execCtx := NewExecutionContext(t.Context(), cmd, inv)

	result := rt.ExecuteCapture(execCtx)
	if result.ExitCode != 0 {
		t.Errorf("ExecuteCapture() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	output := strings.TrimSpace(result.Output)
	if output != "Hello from captured container" {
		t.Errorf("ExecuteCapture() output = %q, want %q", output, "Hello from captured container")
	}
}

// testContainerExecuteCaptureExitCode tests that exit codes are properly captured
func testContainerExecuteCaptureExitCode(t *testing.T) {
	t.Helper()
	_, inv := setupTestInvowkfile(t)

	cmd := &invowkfile.Command{
		Name: "test-capture-exit",
		Implementations: []invowkfile.Implementation{
			{
				Script: "echo 'before exit'; exit 42",
				Runtimes: []invowkfile.RuntimeConfig{
					{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"},
				},
				Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}},
			},
		},
	}

	rt := createContainerRuntime(t)
	execCtx := NewExecutionContext(t.Context(), cmd, inv)

	result := rt.ExecuteCapture(execCtx)
	if result.ExitCode != 42 {
		t.Errorf("ExecuteCapture() exit code = %d, want 42", result.ExitCode)
	}

	// Output should still be captured even with non-zero exit
	if !strings.Contains(result.Output, "before exit") {
		t.Errorf("ExecuteCapture() output should contain 'before exit', got: %q", result.Output)
	}
}

// testContainerExecuteCaptureStderr tests that stderr is captured separately
func testContainerExecuteCaptureStderr(t *testing.T) {
	t.Helper()
	_, inv := setupTestInvowkfile(t)

	cmd := &invowkfile.Command{
		Name: "test-capture-stderr",
		Implementations: []invowkfile.Implementation{
			{
				Script: "echo 'stdout message'; echo 'stderr message' >&2",
				Runtimes: []invowkfile.RuntimeConfig{
					{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"},
				},
				Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}},
			},
		},
	}

	rt := createContainerRuntime(t)
	execCtx := NewExecutionContext(t.Context(), cmd, inv)

	result := rt.ExecuteCapture(execCtx)
	if result.ExitCode != 0 {
		t.Errorf("ExecuteCapture() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	if !strings.Contains(result.Output, "stdout message") {
		t.Errorf("ExecuteCapture() stdout should contain 'stdout message', got: %q", result.Output)
	}

	if !strings.Contains(result.ErrOutput, "stderr message") {
		t.Errorf("ExecuteCapture() stderr should contain 'stderr message', got: %q", result.ErrOutput)
	}
}

// testContainerExecuteCaptureEnvVars tests that environment variables work with capture
func testContainerExecuteCaptureEnvVars(t *testing.T) {
	t.Helper()
	_, inv := setupTestInvowkfile(t)

	currentPlatform := invowkfile.CurrentPlatform()
	cmd := &invowkfile.Command{
		Name: "test-capture-env",
		Implementations: []invowkfile.Implementation{
			{
				Script: `echo "VAR=$MY_VAR"`,
				Runtimes: []invowkfile.RuntimeConfig{
					{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"},
				},
				Platforms: []invowkfile.PlatformConfig{
					{Name: currentPlatform},
				},
				Env: &invowkfile.EnvConfig{Vars: map[string]string{"MY_VAR": "captured_value"}},
			},
		},
	}

	rt := createContainerRuntime(t)
	execCtx := NewExecutionContext(t.Context(), cmd, inv)

	result := rt.ExecuteCapture(execCtx)
	if result.ExitCode != 0 {
		t.Errorf("ExecuteCapture() exit code = %d, want 0, error: %v", result.ExitCode, result.Error)
	}

	if !strings.Contains(result.Output, "VAR=captured_value") {
		t.Errorf("ExecuteCapture() output should contain 'VAR=captured_value', got: %q", result.Output)
	}
}

// TestContainerRuntime_CapturingRuntimeInterface verifies that ContainerRuntime implements CapturingRuntime.
// No container semaphore needed — this is a pure type assertion, no container operations.
func TestContainerRuntime_CapturingRuntimeInterface(t *testing.T) {
	t.Parallel()
	// This is a compile-time check that also serves as documentation
	var _ CapturingRuntime = (*ContainerRuntime)(nil)

	// Also verify at runtime for completeness
	rt := &ContainerRuntime{}
	if _, ok := any(rt).(CapturingRuntime); !ok {
		t.Error("ContainerRuntime does not implement CapturingRuntime interface")
	}
}
