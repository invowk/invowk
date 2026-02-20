// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/container"
)

type (
	// MockSysctlEngine embeds MockEngine and implements container.SysctlOverrideChecker.
	// This simulates a Podman engine that may or may not have the sysctl override active.
	MockSysctlEngine struct {
		*MockEngine
		overrideActive bool
	}

	// MockStderrEngine embeds MockEngine and writes a known string to opts.Stderr
	// on every Run() call, then returns a configurable exit code. This enables
	// testing the stderr buffering and flushing behavior of runWithRetry.
	MockStderrEngine struct {
		*MockEngine
		stderrMsg string
		exitCode  int
	}

	// countingMockEngine is a mock engine that fails with a transient exit code for
	// the first N attempts, then succeeds. It writes distinct stderr messages for
	// failed vs successful attempts to verify that only the correct stderr is flushed.
	countingMockEngine struct {
		*MockEngine
		failUntil     int    // Fail attempts [0, failUntil). Succeed on attempt >= failUntil.
		transientCode int    // Exit code to return on failed attempts.
		failStderr    string // Stderr message for failed attempts.
		successStderr string // Stderr message for the successful attempt.
		attempt       int
	}

	// cancelOnAttemptEngine wraps a MockStderrEngine and cancels a context when
	// a specific attempt index is reached. This simulates external cancellation
	// (e.g., user pressing Ctrl-C) during retry backoff.
	cancelOnAttemptEngine struct {
		*MockStderrEngine
		cancelAtAttempt int
		cancelFunc      context.CancelFunc
		attempt         int
	}
)

// NewMockSysctlEngine creates a MockSysctlEngine with configurable override state.
func NewMockSysctlEngine(overrideActive bool) *MockSysctlEngine {
	return &MockSysctlEngine{
		MockEngine:     NewMockEngine().WithName("podman"),
		overrideActive: overrideActive,
	}
}

// SysctlOverrideActive reports whether the sysctl override temp file is in effect.
func (m *MockSysctlEngine) SysctlOverrideActive() bool {
	return m.overrideActive
}

// Run writes stderrMsg to opts.Stderr and returns a RunResult with the configured
// exit code. This simulates a container engine (e.g., crun) writing diagnostic
// output to stderr before returning a transient exit code.
func (m *MockStderrEngine) Run(_ context.Context, opts container.RunOptions) (*container.RunResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.RunCalls = append(m.RunCalls, opts)

	if opts.Stderr != nil {
		fmt.Fprint(opts.Stderr, m.stderrMsg)
	}
	return &container.RunResult{ExitCode: m.exitCode}, nil
}

func (m *countingMockEngine) Run(_ context.Context, opts container.RunOptions) (*container.RunResult, error) {
	m.mu.Lock()
	currentAttempt := m.attempt
	m.attempt++
	m.RunCalls = append(m.RunCalls, opts)
	m.mu.Unlock()

	if currentAttempt < m.failUntil {
		if opts.Stderr != nil {
			fmt.Fprint(opts.Stderr, m.failStderr)
		}
		return &container.RunResult{ExitCode: m.transientCode}, nil
	}

	if opts.Stderr != nil {
		fmt.Fprint(opts.Stderr, m.successStderr)
	}
	return &container.RunResult{ExitCode: 0}, nil
}

func (m *cancelOnAttemptEngine) Run(ctx context.Context, opts container.RunOptions) (*container.RunResult, error) {
	m.mu.Lock()
	currentAttempt := m.attempt
	m.attempt++
	m.mu.Unlock()

	if currentAttempt == m.cancelAtAttempt {
		m.cancelFunc()
	}

	return m.MockStderrEngine.Run(ctx, opts)
}

// TestRunWithRetry_SerializationDecision verifies the serialization decision
// logic in runWithRetry (lines 214-227 of container_exec.go):
//   - Docker engine (no SysctlOverrideChecker) -> no serialization
//   - Podman with override active -> no serialization
//   - Podman with override inactive -> serialization acquired
//
// All three cases must result in successful execution when the engine returns
// exit code 0.
func TestRunWithRetry_SerializationDecision(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		engine container.Engine
	}{
		{
			name:   "docker engine skips serialization",
			engine: NewMockEngine().WithName("docker"),
		},
		{
			name:   "podman with override active skips serialization",
			engine: NewMockSysctlEngine(true),
		},
		{
			name:   "podman with override inactive acquires serialization",
			engine: NewMockSysctlEngine(false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rt := NewContainerRuntimeWithEngine(tt.engine)
			var stderrBuf bytes.Buffer

			opts := container.RunOptions{
				Image:   "debian:stable-slim",
				Command: []string{"echo", "hello"},
				Stderr:  &stderrBuf,
			}

			result, err := rt.runWithRetry(context.Background(), opts)
			if err != nil {
				t.Fatalf("runWithRetry() returned unexpected error: %v", err)
			}
			if result.ExitCode != 0 {
				t.Errorf("runWithRetry() exit code = %d, want 0", result.ExitCode)
			}
		})
	}
}

// TestRunWithRetry_DockerNoSysctlChecker verifies that a plain MockEngine
// (simulating Docker) does NOT implement SysctlOverrideChecker. This is the
// compile-time/runtime invariant that makes Docker skip serialization.
func TestRunWithRetry_DockerNoSysctlChecker(t *testing.T) {
	t.Parallel()

	var engine container.Engine = NewMockEngine().WithName("docker")
	if _, ok := engine.(container.SysctlOverrideChecker); ok {
		t.Fatal("MockEngine (Docker) must NOT implement SysctlOverrideChecker")
	}
}

// TestRunWithRetry_PodmanImplementsSysctlChecker verifies that MockSysctlEngine
// (simulating Podman) DOES implement SysctlOverrideChecker.
func TestRunWithRetry_PodmanImplementsSysctlChecker(t *testing.T) {
	t.Parallel()

	var engine container.Engine = NewMockSysctlEngine(true)
	checker, ok := engine.(container.SysctlOverrideChecker)
	if !ok {
		t.Fatal("MockSysctlEngine (Podman) must implement SysctlOverrideChecker")
	}
	if !checker.SysctlOverrideActive() {
		t.Error("SysctlOverrideActive() = false, want true")
	}
}

// TestRunWithRetry_StderrFlushedOnExhaustion verifies the C1 fix: when all
// retries are exhausted due to transient exit codes, stderr from the final
// attempt is flushed to the caller's original writer. This ensures the user
// sees diagnostic output from the container engine (e.g., crun error messages)
// even when the operation ultimately fails.
func TestRunWithRetry_StderrFlushedOnExhaustion(t *testing.T) {
	t.Parallel()

	const stderrMsg = "crun: write to /proc/self/setgroups: Permission denied (ping_group_range)"

	engine := &MockStderrEngine{
		MockEngine: NewMockEngine().WithName("mock"),
		stderrMsg:  stderrMsg,
		exitCode:   125, // Transient exit code — triggers retry
	}

	rt := NewContainerRuntimeWithEngine(engine)

	var originalStderr bytes.Buffer
	opts := container.RunOptions{
		Image:   "debian:stable-slim",
		Command: []string{"echo", "hello"},
		Stderr:  &originalStderr,
	}

	result, err := rt.runWithRetry(context.Background(), opts)
	// runWithRetry should return the last result (not an error) when retries
	// exhaust due to transient exit codes (not transient errors).
	if err != nil {
		t.Fatalf("runWithRetry() returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("runWithRetry() returned nil result")
	}
	if result.ExitCode != 125 {
		t.Errorf("runWithRetry() exit code = %d, want 125", result.ExitCode)
	}

	// The final attempt's stderr must be flushed to the original writer.
	got := originalStderr.String()
	if !strings.Contains(got, stderrMsg) {
		t.Errorf("stderr not flushed to original writer\ngot:  %q\nwant: contains %q", got, stderrMsg)
	}

	// Verify all maxRunRetries attempts were made.
	engine.mu.Lock()
	callCount := len(engine.RunCalls)
	engine.mu.Unlock()
	if callCount != maxRunRetries {
		t.Errorf("engine.Run() called %d times, want %d (maxRunRetries)", callCount, maxRunRetries)
	}
}

// TestRunWithRetry_StderrFlushedOnSuccess verifies that on a successful run
// (exit code 0), the stderr buffer from that attempt is flushed to the caller's
// original writer. This ensures warning-level output from the container engine
// is still visible to the user.
func TestRunWithRetry_StderrFlushedOnSuccess(t *testing.T) {
	t.Parallel()

	const stderrMsg = "WARNING: image platform mismatch"

	engine := &MockStderrEngine{
		MockEngine: NewMockEngine().WithName("mock"),
		stderrMsg:  stderrMsg,
		exitCode:   0, // Success on first attempt
	}

	rt := NewContainerRuntimeWithEngine(engine)

	var originalStderr bytes.Buffer
	opts := container.RunOptions{
		Image:   "debian:stable-slim",
		Command: []string{"echo", "hello"},
		Stderr:  &originalStderr,
	}

	result, err := rt.runWithRetry(context.Background(), opts)
	if err != nil {
		t.Fatalf("runWithRetry() returned unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("runWithRetry() exit code = %d, want 0", result.ExitCode)
	}

	got := originalStderr.String()
	if !strings.Contains(got, stderrMsg) {
		t.Errorf("stderr not flushed on success\ngot:  %q\nwant: contains %q", got, stderrMsg)
	}

	// Should succeed on the first attempt with no retries.
	engine.mu.Lock()
	callCount := len(engine.RunCalls)
	engine.mu.Unlock()
	if callCount != 1 {
		t.Errorf("engine.Run() called %d times, want 1", callCount)
	}
}

// TestRunWithRetry_StderrNotLeakedOnTransientRetry verifies that stderr from
// intermediate transient-failure attempts is NOT flushed to the caller when a
// subsequent attempt succeeds. Only the successful attempt's stderr is flushed.
func TestRunWithRetry_StderrNotLeakedOnTransientRetry(t *testing.T) {
	t.Parallel()

	engine := NewMockEngine().WithName("mock")
	rt := NewContainerRuntimeWithEngine(engine)

	// Replace the engine with a counting mock that fails on the first attempt
	// with a transient exit code, then succeeds on the second attempt.
	countingEngine := &countingMockEngine{
		MockEngine:    engine,
		failUntil:     1, // Fail attempt 0, succeed on attempt 1
		transientCode: 125,
		failStderr:    "transient crun error from attempt 0",
		successStderr: "success warning from attempt 1",
	}
	rt.engine = countingEngine

	var originalStderr bytes.Buffer
	opts := container.RunOptions{
		Image:   "debian:stable-slim",
		Command: []string{"echo", "hello"},
		Stderr:  &originalStderr,
	}

	result, err := rt.runWithRetry(context.Background(), opts)
	if err != nil {
		t.Fatalf("runWithRetry() returned unexpected error: %v", err)
	}
	if result.ExitCode != 0 {
		t.Errorf("runWithRetry() exit code = %d, want 0", result.ExitCode)
	}

	got := originalStderr.String()

	// The transient error's stderr must NOT appear in the output.
	if strings.Contains(got, "transient crun error") {
		t.Errorf("transient attempt stderr leaked to original writer: %q", got)
	}

	// The successful attempt's stderr SHOULD appear.
	if !strings.Contains(got, "success warning from attempt 1") {
		t.Errorf("successful attempt stderr not flushed\ngot:  %q\nwant: contains %q", got, "success warning from attempt 1")
	}
}

// TestRunWithRetry_ContextCancelled verifies that runWithRetry returns
// immediately when the context is cancelled between retry attempts, without
// waiting for the full backoff.
func TestRunWithRetry_ContextCancelled(t *testing.T) {
	t.Parallel()

	engine := &MockStderrEngine{
		MockEngine: NewMockEngine().WithName("mock"),
		stderrMsg:  "transient error",
		exitCode:   126, // Transient exit code
	}

	rt := NewContainerRuntimeWithEngine(engine)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context after the first attempt completes.
	// We use a custom engine wrapper that cancels on the second call.
	cancellingEngine := &cancelOnAttemptEngine{
		MockStderrEngine: engine,
		cancelAtAttempt:  1,
		cancelFunc:       cancel,
	}
	rt.engine = cancellingEngine

	var originalStderr bytes.Buffer
	opts := container.RunOptions{
		Image:   "debian:stable-slim",
		Command: []string{"echo", "hello"},
		Stderr:  &originalStderr,
	}

	_, err := rt.runWithRetry(ctx, opts)
	if err == nil {
		t.Fatal("runWithRetry() should return error when context is cancelled")
	}
	if !strings.Contains(err.Error(), "context cancelled") {
		t.Errorf("error should mention context cancellation, got: %v", err)
	}
}

// TestIsTransientExitCode verifies the exit code classification used by
// runWithRetry to decide whether to retry a container run.
func TestIsTransientExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code int
		want bool
	}{
		{0, false},
		{1, false},
		{2, false},
		{125, true},
		{126, true},
		{127, false},
		{137, false},
		{255, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("exit_code_%d", tt.code), func(t *testing.T) {
			t.Parallel()
			if got := IsTransientExitCode(tt.code); got != tt.want {
				t.Errorf("IsTransientExitCode(%d) = %v, want %v", tt.code, got, tt.want)
			}
		})
	}
}

// TestFlushStderr verifies the flushStderr helper handles edge cases correctly.
func TestFlushStderr(t *testing.T) {
	t.Parallel()

	t.Run("nil destination is no-op", func(t *testing.T) {
		t.Parallel()
		src := bytes.NewBufferString("some output")
		// Should not panic.
		flushStderr(nil, src)
	})

	t.Run("empty source is no-op", func(t *testing.T) {
		t.Parallel()
		var dst bytes.Buffer
		src := &bytes.Buffer{}
		flushStderr(&dst, src)
		if dst.Len() != 0 {
			t.Errorf("destination should be empty, got %q", dst.String())
		}
	})

	t.Run("content is copied", func(t *testing.T) {
		t.Parallel()
		var dst bytes.Buffer
		src := bytes.NewBufferString("error output")
		flushStderr(&dst, src)
		if dst.String() != "error output" {
			t.Errorf("destination = %q, want %q", dst.String(), "error output")
		}
	})
}
