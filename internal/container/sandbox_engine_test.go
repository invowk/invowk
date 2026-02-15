// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"io"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/platform"
)

var _ io.Writer = (*recordingWriter)(nil)

type (
	// mockEngine implements Engine interface for testing
	mockEngine struct {
		name       string
		available  bool
		binaryPath string
		buildArgs  []string
	}

	// recordingWriter records what was written for testing
	recordingWriter struct {
		data []byte
	}
)

func (m *mockEngine) Name() string {
	return m.name
}

func (m *mockEngine) Available() bool {
	return m.available
}

func (m *mockEngine) Version(_ context.Context) (string, error) {
	return "1.0.0", nil
}

func (m *mockEngine) BinaryPath() string {
	return m.binaryPath
}

func (m *mockEngine) BuildRunArgs(_ RunOptions) []string {
	if m.buildArgs != nil {
		return m.buildArgs
	}
	return []string{"run", "--rm", "debian:stable-slim", "echo", "hello"}
}

func (m *mockEngine) Build(_ context.Context, _ BuildOptions) error {
	return nil
}

func (m *mockEngine) Run(_ context.Context, _ RunOptions) (*RunResult, error) {
	return &RunResult{}, nil
}

func (m *mockEngine) Remove(_ context.Context, _ string, _ bool) error {
	return nil
}

func (m *mockEngine) ImageExists(_ context.Context, _ string) (bool, error) {
	return true, nil
}

func (m *mockEngine) RemoveImage(_ context.Context, _ string, _ bool) error {
	return nil
}

func (w *recordingWriter) Write(p []byte) (n int, err error) {
	w.data = append(w.data, p...)
	return len(p), nil
}

func TestSandboxAwareEngine_NoSandbox(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{
		name:       "podman",
		available:  true,
		binaryPath: "/usr/bin/podman",
		buildArgs:  []string{"run", "--rm", "test-image"},
	}

	// Create engine with no sandbox
	engine := newSandboxAwareEngineForTesting(mock, platform.SandboxNone)

	// BuildRunArgs should return args unchanged
	args := engine.BuildRunArgs(RunOptions{})
	expected := []string{"run", "--rm", "test-image"}

	if !slices.Equal(args, expected) {
		t.Errorf("BuildRunArgs() = %v, want %v", args, expected)
	}
}

func TestSandboxAwareEngine_Flatpak(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{
		name:       "podman",
		available:  true,
		binaryPath: "/usr/bin/podman",
		buildArgs:  []string{"run", "--rm", "test-image"},
	}

	// Create engine with Flatpak sandbox
	engine := newSandboxAwareEngineForTesting(mock, platform.SandboxFlatpak)

	// BuildRunArgs should prepend flatpak-spawn --host
	args := engine.BuildRunArgs(RunOptions{})
	expected := []string{"flatpak-spawn", "--host", "/usr/bin/podman", "run", "--rm", "test-image"}

	if !slices.Equal(args, expected) {
		t.Errorf("BuildRunArgs() = %v, want %v", args, expected)
	}
}

func TestSandboxAwareEngine_Snap(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{
		name:       "docker",
		available:  true,
		binaryPath: "/usr/bin/docker",
		buildArgs:  []string{"run", "--rm", "test-image"},
	}

	// Create engine with Snap sandbox
	engine := newSandboxAwareEngineForTesting(mock, platform.SandboxSnap)

	// BuildRunArgs should prepend snap run --shell
	args := engine.BuildRunArgs(RunOptions{})
	expected := []string{"snap", "run", "--shell", "/usr/bin/docker", "run", "--rm", "test-image"}

	if !slices.Equal(args, expected) {
		t.Errorf("BuildRunArgs() = %v, want %v", args, expected)
	}
}

func TestSandboxAwareEngine_DelegatesMethods(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{
		name:       "podman",
		available:  true,
		binaryPath: "/usr/bin/podman",
	}

	// Test with no sandbox to ensure proper delegation
	engine := newSandboxAwareEngineForTesting(mock, platform.SandboxNone)

	// Name should delegate
	if engine.Name() != "podman" {
		t.Errorf("Name() = %q, want %q", engine.Name(), "podman")
	}

	// Available should delegate
	if !engine.Available() {
		t.Error("Available() = false, want true")
	}

	// BinaryPath should delegate
	if engine.BinaryPath() != "/usr/bin/podman" {
		t.Errorf("BinaryPath() = %q, want %q", engine.BinaryPath(), "/usr/bin/podman")
	}

	// Version should delegate
	version, err := engine.Version(context.Background())
	if err != nil {
		t.Errorf("Version() error = %v", err)
	}
	if version != "1.0.0" {
		t.Errorf("Version() = %q, want %q", version, "1.0.0")
	}
}

func TestSandboxAwareEngine_BuildSpawnArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		sandboxType platform.SandboxType
		binary      string
		args        []string
		expected    []string
	}{
		{
			name:        "flatpak simple",
			sandboxType: platform.SandboxFlatpak,
			binary:      "/usr/bin/podman",
			args:        []string{"run", "--rm", "debian:stable-slim"},
			expected:    []string{"flatpak-spawn", "--host", "/usr/bin/podman", "run", "--rm", "debian:stable-slim"},
		},
		{
			name:        "flatpak with volume",
			sandboxType: platform.SandboxFlatpak,
			binary:      "/usr/bin/podman",
			args:        []string{"run", "-v", "/tmp/test:/workspace", "debian:stable-slim"},
			expected:    []string{"flatpak-spawn", "--host", "/usr/bin/podman", "run", "-v", "/tmp/test:/workspace", "debian:stable-slim"},
		},
		{
			name:        "snap simple",
			sandboxType: platform.SandboxSnap,
			binary:      "/snap/bin/docker",
			args:        []string{"build", "-t", "myimage", "."},
			expected:    []string{"snap", "run", "--shell", "/snap/bin/docker", "build", "-t", "myimage", "."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockEngine{binaryPath: tt.binary}
			engine := newSandboxAwareEngineForTesting(mock, tt.sandboxType)

			result := engine.buildSpawnArgs(tt.binary, tt.args)

			if !slices.Equal(result, tt.expected) {
				t.Errorf("buildSpawnArgs() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSandboxAwareEngine_WrapArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		sandboxType platform.SandboxType
		args        []string
		wantWrapped bool
	}{
		{
			name:        "no sandbox - no wrap",
			sandboxType: platform.SandboxNone,
			args:        []string{"run", "--rm", "debian:stable-slim"},
			wantWrapped: false,
		},
		{
			name:        "flatpak - wrap",
			sandboxType: platform.SandboxFlatpak,
			args:        []string{"run", "--rm", "debian:stable-slim"},
			wantWrapped: true,
		},
		{
			name:        "snap - wrap",
			sandboxType: platform.SandboxSnap,
			args:        []string{"run", "--rm", "debian:stable-slim"},
			wantWrapped: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mock := &mockEngine{binaryPath: "/usr/bin/podman"}
			engine := newSandboxAwareEngineForTesting(mock, tt.sandboxType)

			result := engine.wrapArgs(tt.args)

			if tt.wantWrapped {
				// Should have spawn command prepended
				if result[0] == tt.args[0] {
					t.Errorf("wrapArgs() should wrap args, got %v", result)
				}
			} else {
				// Should be unchanged
				if !slices.Equal(result, tt.args) {
					t.Errorf("wrapArgs() = %v, want %v", result, tt.args)
				}
			}
		})
	}
}

func TestNewSandboxAwareEngine_NoSandbox(t *testing.T) {
	t.Parallel()

	// Reset sandbox detection
	platform.DetectSandbox() // Ensure detection runs

	mock := &mockEngine{
		name:       "test",
		available:  true,
		binaryPath: "/usr/bin/test",
	}

	// When not in a sandbox, should return the original engine unwrapped
	engine := NewSandboxAwareEngine(mock)

	// If we're actually in a sandbox, it will be wrapped
	if platform.IsInSandbox() {
		// Verify it's wrapped
		if _, ok := engine.(*SandboxAwareEngine); !ok {
			t.Error("expected SandboxAwareEngine when in sandbox")
		}
	} else {
		// Should be the original mock, not wrapped
		if engine != mock {
			t.Error("expected original engine when not in sandbox")
		}
	}
}

func TestSandboxAwareEngine_ComplexRunOptions(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{
		name:       "podman",
		available:  true,
		binaryPath: "/usr/bin/podman",
		buildArgs: []string{
			"run", "--rm", "-i", "-t",
			"-w", "/workspace",
			"-v", "/home/user/project:/workspace:z",
			"-e", "FOO=bar",
			"--userns=keep-id",
			"debian:stable-slim",
			"bash", "-c", "echo hello",
		},
	}

	engine := newSandboxAwareEngineForTesting(mock, platform.SandboxFlatpak)

	args := engine.BuildRunArgs(RunOptions{
		Image:       "debian:stable-slim",
		Command:     []string{"bash", "-c", "echo hello"},
		WorkDir:     "/workspace",
		Volumes:     []string{"/home/user/project:/workspace:z"},
		Env:         map[string]string{"FOO": "bar"},
		Remove:      true,
		Interactive: true,
		TTY:         true,
	})

	// Verify flatpak-spawn --host is prepended
	if len(args) < 3 {
		t.Fatalf("expected at least 3 args, got %d", len(args))
	}

	if args[0] != "flatpak-spawn" {
		t.Errorf("args[0] = %q, want %q", args[0], "flatpak-spawn")
	}
	if args[1] != "--host" {
		t.Errorf("args[1] = %q, want %q", args[1], "--host")
	}
	if args[2] != "/usr/bin/podman" {
		t.Errorf("args[2] = %q, want %q", args[2], "/usr/bin/podman")
	}

	// Verify volume mount is preserved (this is the key fix!)
	foundVolume := false
	for i, arg := range args {
		if arg == "-v" && i+1 < len(args) && args[i+1] == "/home/user/project:/workspace:z" {
			foundVolume = true
			break
		}
	}
	if !foundVolume {
		t.Error("volume mount not found in wrapped args")
	}
}

func TestSandboxAwareEngine_CustomizeCmd_Propagates(t *testing.T) {
	t.Parallel()

	// Create an engine with env overrides
	engine := NewBaseCLIEngine("/usr/bin/podman",
		WithCmdEnvOverride("TEST_OVERRIDE", "yes"),
	)
	podman := &PodmanEngine{BaseCLIEngine: engine}

	// Wrap in SandboxAwareEngine
	wrapper := newSandboxAwareEngineForTesting(podman, platform.SandboxFlatpak)

	cmd := exec.CommandContext(context.Background(), "true")
	wrapper.CustomizeCmd(cmd)

	if !slices.ContainsFunc(cmd.Env, func(s string) bool {
		return strings.HasPrefix(s, "TEST_OVERRIDE=yes")
	}) {
		t.Error("SandboxAwareEngine.CustomizeCmd should propagate env overrides from wrapped engine")
	}
}

func TestSandboxAwareEngine_SysctlOverrideActive_Forwards(t *testing.T) {
	t.Parallel()

	t.Run("podman with active override", func(t *testing.T) {
		t.Parallel()
		podman := &PodmanEngine{
			BaseCLIEngine: NewBaseCLIEngine("/usr/bin/podman",
				WithSysctlOverrideActive(true),
			),
		}
		wrapper := newSandboxAwareEngineForTesting(podman, platform.SandboxFlatpak)
		if !wrapper.SysctlOverrideActive() {
			t.Error("SandboxAwareEngine should forward SysctlOverrideActive=true from PodmanEngine")
		}
	})

	t.Run("podman without override", func(t *testing.T) {
		t.Parallel()
		podman := &PodmanEngine{
			BaseCLIEngine: NewBaseCLIEngine("/usr/bin/podman-remote"),
		}
		wrapper := newSandboxAwareEngineForTesting(podman, platform.SandboxFlatpak)
		if wrapper.SysctlOverrideActive() {
			t.Error("SandboxAwareEngine should forward SysctlOverrideActive=false from PodmanEngine")
		}
	})

	t.Run("mock engine without checker", func(t *testing.T) {
		t.Parallel()
		mock := &mockEngine{}
		wrapper := newSandboxAwareEngineForTesting(mock, platform.SandboxFlatpak)
		if wrapper.SysctlOverrideActive() {
			t.Error("SandboxAwareEngine should return false for engines without SysctlOverrideChecker")
		}
	})
}

func TestSandboxAwareEngine_GetBaseCLIEngine(t *testing.T) {
	t.Parallel()

	// Test with real PodmanEngine
	podman := NewPodmanEngine()
	podmanWrapper := newSandboxAwareEngineForTesting(podman, platform.SandboxFlatpak)

	base, ok := podmanWrapper.getBaseCLIEngine()
	if !ok {
		t.Error("getBaseCLIEngine should return true for PodmanEngine")
	}
	if base == nil {
		t.Error("getBaseCLIEngine should return non-nil BaseCLIEngine for PodmanEngine")
	}

	// Test with real DockerEngine
	docker := NewDockerEngine()
	dockerWrapper := newSandboxAwareEngineForTesting(docker, platform.SandboxFlatpak)

	base, ok = dockerWrapper.getBaseCLIEngine()
	if !ok {
		t.Error("getBaseCLIEngine should return true for DockerEngine")
	}
	if base == nil {
		t.Error("getBaseCLIEngine should return non-nil BaseCLIEngine for DockerEngine")
	}

	// Test with mock (unknown type)
	mock := &mockEngine{}
	mockWrapper := newSandboxAwareEngineForTesting(mock, platform.SandboxFlatpak)

	_, ok = mockWrapper.getBaseCLIEngine()
	if ok {
		t.Error("getBaseCLIEngine should return false for unknown engine types")
	}
}
