// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/pkg/invowkfile"
)

type cancelBlockingMockEngine struct {
	*MockEngine
	started chan struct{}
}

func (m *cancelBlockingMockEngine) Run(ctx context.Context, opts container.RunOptions) (*container.RunResult, error) {
	m.mu.Lock()
	m.RunCalls = append(m.RunCalls, opts)
	m.mu.Unlock()

	if m.started != nil {
		select {
		case <-m.started:
		default:
			close(m.started)
		}
	}

	<-ctx.Done()
	return nil, fmt.Errorf("mock container run cancelled: %w", ctx.Err())
}

func findHostTempScript(t *testing.T, invowkDir string, command []string) string {
	t.Helper()
	for _, arg := range command {
		if strings.HasPrefix(arg, "/workspace/invowk-script-") {
			return filepath.Join(invowkDir, strings.TrimPrefix(arg, "/workspace/"))
		}
	}
	return ""
}

func TestContainerRuntime_CancelCleanup(t *testing.T) {
	t.Parallel()

	t.Run("Execute stops and removes ephemeral container before temp script cleanup", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		inv := &invowkfile.Invowkfile{
			FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
		}
		cmd := &invowkfile.Command{
			Name: "ephem-exec",
			Implementations: []invowkfile.Implementation{{
				Script: invowkfile.ImplementationScript{
					Content:     "echo sleeping && sleep 30",
					Interpreter: "sh",
				},
				Runtimes: []invowkfile.RuntimeConfig{{
					Name:  invowkfile.RuntimeContainer,
					Image: "debian:stable-slim",
				}},
				Platforms: invowkfile.AllPlatformConfigs(),
			}},
		}

		var tempScriptAtStop string
		var tempScriptExistedAtStop bool
		var tempScriptExistedAtRemove bool
		baseMock := NewMockEngine()
		baseMock.OnStop = func(_ context.Context, _ container.ContainerID, _ time.Duration) error {
			if len(baseMock.RunCalls) > 0 {
				tempScriptAtStop = findHostTempScript(t, tmpDir, baseMock.RunCalls[0].Command)
				if tempScriptAtStop != "" {
					_, statErr := os.Stat(tempScriptAtStop)
					tempScriptExistedAtStop = statErr == nil
				}
			}
			return nil
		}
		baseMock.OnRemove = func(_ context.Context, _ container.ContainerID, _ bool) error {
			if tempScriptAtStop != "" {
				_, statErr := os.Stat(tempScriptAtStop)
				tempScriptExistedAtRemove = statErr == nil
			}
			return nil
		}

		engine := &cancelBlockingMockEngine{
			MockEngine: baseMock,
			started:    make(chan struct{}),
		}
		rt, err := NewContainerRuntimeWithEngine(engine)
		if err != nil {
			t.Fatalf("NewContainerRuntimeWithEngine() error = %v", err)
		}

		cancelCtx, cancel := context.WithCancel(t.Context())
		defer cancel()

		execCtx := NewExecutionContext(cancelCtx, cmd, inv)
		execCtx.SelectedRuntime = invowkfile.RuntimeContainer
		execCtx.SelectedImpl = &cmd.Implementations[0]
		execCtx.ExecutionID = "123456-1"

		resCh := make(chan *Result, 1)
		go func() {
			resCh <- rt.Execute(execCtx)
		}()

		<-engine.started
		cancel()
		result := <-resCh

		if !errors.Is(result.Error, context.Canceled) {
			t.Fatalf("Execute() error = %v, want context.Canceled", result.Error)
		}
		if len(baseMock.RunCalls) != 1 {
			t.Fatalf("RunCalls = %d, want 1", len(baseMock.RunCalls))
		}
		runName := baseMock.RunCalls[0].Name
		if !strings.HasPrefix(string(runName), ephemeralContainerNamePrefix) {
			t.Fatalf("RunCalls[0].Name = %q, want prefix %q", runName, ephemeralContainerNamePrefix)
		}
		if len(baseMock.StopCalls) != 1 {
			t.Fatalf("StopCalls = %d, want 1", len(baseMock.StopCalls))
		}
		stopCall := baseMock.StopCalls[0]
		if stopCall.ContainerID != container.ContainerID(runName) {
			t.Fatalf("Stop ContainerID = %q, want %q", stopCall.ContainerID, runName)
		}
		if stopCall.CtxErr != nil || !stopCall.HasDeadline {
			t.Fatalf("Stop ctx = (err=%v, hasDeadline=%v), want (nil, true)", stopCall.CtxErr, stopCall.HasDeadline)
		}
		if len(baseMock.RemoveCalls) != 1 {
			t.Fatalf("RemoveCalls = %d, want 1", len(baseMock.RemoveCalls))
		}
		rmCall := baseMock.RemoveCalls[0]
		if rmCall.ContainerID != container.ContainerID(runName) || !rmCall.Force {
			t.Fatalf("Remove = (%q, force=%v), want (%q, true)", rmCall.ContainerID, rmCall.Force, runName)
		}
		if rmCall.CtxErr != nil || !rmCall.HasDeadline {
			t.Fatalf("Remove ctx = (err=%v, hasDeadline=%v), want (nil, true)", rmCall.CtxErr, rmCall.HasDeadline)
		}
		if tempScriptAtStop == "" || !tempScriptExistedAtStop || !tempScriptExistedAtRemove {
			t.Fatalf("temp script must exist during Stop/Remove (path=%q, atStop=%v, atRemove=%v)",
				tempScriptAtStop, tempScriptExistedAtStop, tempScriptExistedAtRemove)
		}
		if _, statErr := os.Stat(tempScriptAtStop); !errors.Is(statErr, fs.ErrNotExist) {
			t.Fatalf("temp script %q still exists after Execute() cleanup (stat err = %v)", tempScriptAtStop, statErr)
		}
	})

	t.Run("ExecuteCapture stops and removes ephemeral container on cancellation", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		inv := &invowkfile.Invowkfile{
			FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
		}
		cmd := &invowkfile.Command{
			Name: "ephem-capture",
			Implementations: []invowkfile.Implementation{{
				Script: invowkfile.ImplementationScript{Content: "sleep 30"},
				Runtimes: []invowkfile.RuntimeConfig{{
					Name:  invowkfile.RuntimeContainer,
					Image: "debian:stable-slim",
				}},
				Platforms: invowkfile.AllPlatformConfigs(),
			}},
		}

		baseMock := NewMockEngine()
		engine := &cancelBlockingMockEngine{
			MockEngine: baseMock,
			started:    make(chan struct{}),
		}
		rt, err := NewContainerRuntimeWithEngine(engine)
		if err != nil {
			t.Fatalf("NewContainerRuntimeWithEngine() error = %v", err)
		}

		cancelCtx, cancel := context.WithCancel(t.Context())
		defer cancel()

		execCtx := NewExecutionContext(cancelCtx, cmd, inv)
		execCtx.SelectedRuntime = invowkfile.RuntimeContainer
		execCtx.SelectedImpl = &cmd.Implementations[0]
		execCtx.ExecutionID = "123456-2"

		resCh := make(chan *Result, 1)
		go func() {
			resCh <- rt.ExecuteCapture(execCtx)
		}()

		<-engine.started
		cancel()
		result := <-resCh

		if !errors.Is(result.Error, context.Canceled) {
			t.Fatalf("ExecuteCapture() error = %v, want context.Canceled", result.Error)
		}
		if len(baseMock.RunCalls) != 1 {
			t.Fatalf("RunCalls = %d, want 1", len(baseMock.RunCalls))
		}
		runName := baseMock.RunCalls[0].Name
		if !strings.HasPrefix(string(runName), ephemeralContainerNamePrefix) {
			t.Fatalf("RunCalls[0].Name = %q, want prefix %q", runName, ephemeralContainerNamePrefix)
		}
		if len(baseMock.StopCalls) != 1 || len(baseMock.RemoveCalls) != 1 {
			t.Fatalf("StopCalls=%d RemoveCalls=%d, want 1 and 1", len(baseMock.StopCalls), len(baseMock.RemoveCalls))
		}
	})

	t.Run("PrepareCommand cleanup stops and removes cancelled interactive ephemeral container before temp script cleanup", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		inv := &invowkfile.Invowkfile{
			FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue")),
		}
		cmd := &invowkfile.Command{
			Name: "ephem-prepare",
			Implementations: []invowkfile.Implementation{{
				Script: invowkfile.ImplementationScript{
					Content:     "echo interactive",
					Interpreter: "sh",
				},
				Runtimes: []invowkfile.RuntimeConfig{{
					Name:  invowkfile.RuntimeContainer,
					Image: "debian:stable-slim",
				}},
				Platforms: invowkfile.AllPlatformConfigs(),
			}},
		}

		var tempScriptPath string
		var tempScriptExistedAtStop bool
		baseMock := NewMockEngine()
		baseMock.OnStop = func(_ context.Context, _ container.ContainerID, _ time.Duration) error {
			if tempScriptPath != "" {
				_, statErr := os.Stat(tempScriptPath)
				tempScriptExistedAtStop = statErr == nil
			}
			return nil
		}
		rt, err := NewContainerRuntimeWithEngine(baseMock)
		if err != nil {
			t.Fatalf("NewContainerRuntimeWithEngine() error = %v", err)
		}

		cancelCtx, cancel := context.WithCancel(t.Context())
		execCtx := NewExecutionContext(cancelCtx, cmd, inv)
		execCtx.SelectedRuntime = invowkfile.RuntimeContainer
		execCtx.SelectedImpl = &cmd.Implementations[0]
		execCtx.ExecutionID = "123456-3"

		prepared, err := rt.PrepareCommand(execCtx)
		if err != nil {
			t.Fatalf("PrepareCommand() error = %v", err)
		}
		if len(baseMock.PrepareRunCalls) != 1 {
			t.Fatalf("PrepareRunCalls = %d, want 1", len(baseMock.PrepareRunCalls))
		}
		prepOpts := baseMock.PrepareRunCalls[0]
		if !strings.HasPrefix(string(prepOpts.Name), ephemeralContainerNamePrefix) {
			t.Fatalf("PrepareRunCalls[0].Name = %q, want prefix %q", prepOpts.Name, ephemeralContainerNamePrefix)
		}
		tempScriptPath = findHostTempScript(t, tmpDir, prepOpts.Command)

		cancel()
		prepared.Cleanup()

		if len(baseMock.StopCalls) != 1 || len(baseMock.RemoveCalls) != 1 {
			t.Fatalf("StopCalls=%d RemoveCalls=%d, want 1 and 1", len(baseMock.StopCalls), len(baseMock.RemoveCalls))
		}
		if baseMock.StopCalls[0].CtxErr != nil || !baseMock.StopCalls[0].HasDeadline {
			t.Fatalf("Stop ctx = (err=%v, hasDeadline=%v), want (nil, true)",
				baseMock.StopCalls[0].CtxErr, baseMock.StopCalls[0].HasDeadline)
		}
		if tempScriptPath == "" || !tempScriptExistedAtStop {
			t.Fatalf("temp script must exist during Stop (path=%q, atStop=%v)", tempScriptPath, tempScriptExistedAtStop)
		}
		if _, statErr := os.Stat(tempScriptPath); !errors.Is(statErr, fs.ErrNotExist) {
			t.Fatalf("temp script %q still exists after Cleanup() (stat err = %v)", tempScriptPath, statErr)
		}
	})

	t.Run("persistent container is not stopped or removed on cancellation", func(t *testing.T) {
		t.Parallel()

		engine := NewMockEngine().WithInspectInfo(&container.ContainerInfo{
			ContainerID: "persist-123",
			Name:        "dev-persist",
			Running:     true,
		})
		rt := newPersistentTestRuntime(t, engine)

		cancelCtx, cancel := context.WithCancel(t.Context())
		execCtx := newPersistentExecutionContext(t, nil)
		execCtx.Context = cancelCtx
		execCtx.ContainerNameOverride = "dev-persist"

		cancel()
		result := rt.Execute(execCtx)
		if !errors.Is(result.Error, context.Canceled) {
			t.Fatalf("Execute() error = %v, want context.Canceled", result.Error)
		}
		if len(engine.StopCalls) != 0 || len(engine.RemoveCalls) != 0 {
			t.Fatalf("persistent container should not be stopped/removed (stop=%d, remove=%d)",
				len(engine.StopCalls), len(engine.RemoveCalls))
		}
	})
}
