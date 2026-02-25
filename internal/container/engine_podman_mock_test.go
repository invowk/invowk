// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

// =============================================================================
// Podman Engine Mock Tests (T073)
// =============================================================================

// newTestPodmanEngine creates a PodmanEngine for testing with the mock recorder.
// Note: SELinux volume labeling is disabled in tests to simplify assertions.
func newTestPodmanEngine(t *testing.T, recorder *MockCommandRecorder) *PodmanEngine {
	t.Helper()
	return &PodmanEngine{
		BaseCLIEngine: NewBaseCLIEngine("/usr/bin/podman", WithExecCommand(recorder.ContextCommandFunc(t))),
	}
}

// TestPodmanEngine_Build_Arguments verifies Podman Build() constructs correct arguments.
func TestPodmanEngine_Build_Arguments(t *testing.T) {
	t.Parallel()

	t.Run("basic build", func(t *testing.T) {
		t.Parallel()
		recorder := NewMockCommandRecorder()
		engine := newTestPodmanEngine(t, recorder)
		ctx := context.Background()

		opts := BuildOptions{
			ContextDir: "/tmp/build",
			Tag:        "myimage:latest",
		}

		err := engine.Build(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertInvocationCount(t, 1)
		recorder.AssertCommandName(t, "/usr/bin/podman")
		recorder.AssertFirstArg(t, "build")
		recorder.AssertArgsContain(t, "-t")
		recorder.AssertArgsContain(t, "myimage:latest")
	})

	t.Run("with no-cache", func(t *testing.T) {
		t.Parallel()
		recorder := NewMockCommandRecorder()
		engine := newTestPodmanEngine(t, recorder)
		ctx := context.Background()

		opts := BuildOptions{
			ContextDir: "/tmp/build",
			Tag:        "test:v1",
			NoCache:    true,
		}

		err := engine.Build(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "--no-cache")
	})
}

// TestPodmanEngine_Run_Arguments verifies Podman Run() constructs correct arguments.
func TestPodmanEngine_Run_Arguments(t *testing.T) {
	t.Parallel()

	t.Run("basic run", func(t *testing.T) {
		t.Parallel()
		recorder := NewMockCommandRecorder()
		engine := newTestPodmanEngine(t, recorder)
		ctx := context.Background()

		opts := RunOptions{
			Image:   "debian:stable-slim",
			Command: []string{"echo", "hello"},
		}

		_, err := engine.Run(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertInvocationCount(t, 1)
		recorder.AssertCommandName(t, "/usr/bin/podman")
		recorder.AssertFirstArg(t, "run")
		recorder.AssertArgsContain(t, "debian:stable-slim")
	})

	t.Run("with all options", func(t *testing.T) {
		t.Parallel()
		recorder := NewMockCommandRecorder()
		engine := newTestPodmanEngine(t, recorder)
		ctx := context.Background()

		opts := RunOptions{
			Image:       "debian:stable-slim",
			Command:     []string{"bash", "-c", "echo test"},
			WorkDir:     "/app",
			Name:        "podman-test",
			Remove:      true,
			Interactive: true,
			TTY:         true,
			Env:         map[string]string{"VAR": "value"},
			Volumes:     []invowkfile.VolumeMountSpec{"/src:/src"},
			Ports:       []invowkfile.PortMappingSpec{"8080:80"},
			ExtraHosts:  []HostMapping{"host.containers.internal:host-gateway"},
		}

		_, err := engine.Run(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "--rm")
		recorder.AssertArgsContain(t, "--name")
		recorder.AssertArgsContain(t, "podman-test")
		recorder.AssertArgsContain(t, "-w")
		recorder.AssertArgsContain(t, "/app")
		recorder.AssertArgsContain(t, "-i")
		recorder.AssertArgsContain(t, "-t")
		recorder.AssertArgsContain(t, "-e")
		recorder.AssertArgsContain(t, "-v")
		recorder.AssertArgsContain(t, "-p")
		recorder.AssertArgsContain(t, "8080:80")
		recorder.AssertArgsContain(t, "--add-host")
	})
}

// TestPodmanEngine_ImageExists_Arguments verifies Podman uses 'image exists' not 'image inspect'.
func TestPodmanEngine_ImageExists_Arguments(t *testing.T) {
	t.Parallel()

	t.Run("podman uses image exists command", func(t *testing.T) {
		t.Parallel()
		recorder := NewMockCommandRecorder()
		engine := newTestPodmanEngine(t, recorder)
		ctx := context.Background()

		exists, err := engine.ImageExists(ctx, "myimage:latest")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !exists {
			t.Error("expected image to exist (mock returns success)")
		}

		recorder.AssertInvocationCount(t, 1)
		recorder.AssertCommandName(t, "/usr/bin/podman")
		recorder.AssertFirstArg(t, "image")
		// Note: Podman uses "exists" while Docker uses "inspect"
		recorder.AssertArgsContain(t, "exists")
		recorder.AssertArgsContain(t, "myimage:latest")
	})
}

// TestPodmanEngine_ErrorPaths verifies Podman error handling.
func TestPodmanEngine_ErrorPaths(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("build failure", func(t *testing.T) {
		t.Parallel()

		recorder := NewMockCommandRecorder()
		recorder.Stderr = "Error: build failed"
		recorder.ExitCode = 1
		engine := newTestPodmanEngine(t, recorder)

		opts := BuildOptions{
			ContextDir: "/tmp/build",
			Tag:        "test:v1",
		}

		err := engine.Build(ctx, opts)
		if err == nil {
			t.Fatal("expected error for failed build")
		}
		// Build now returns an actionable error with "failed to build container image" operation
		if !strings.Contains(err.Error(), "failed to build container image") {
			t.Errorf("expected 'failed to build container image' error, got: %v", err)
		}
	})

	t.Run("image not found", func(t *testing.T) {
		t.Parallel()

		recorder := NewMockCommandRecorder()
		recorder.Stderr = "Error: No such image"
		recorder.ExitCode = 1
		engine := newTestPodmanEngine(t, recorder)

		exists, err := engine.ImageExists(ctx, "nonexistent:latest")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if exists {
			t.Error("expected image to not exist")
		}
	})

	t.Run("run with exit code", func(t *testing.T) {
		t.Parallel()

		recorder := NewMockCommandRecorder()
		recorder.Stderr = "command failed"
		recorder.ExitCode = 42
		engine := newTestPodmanEngine(t, recorder)

		opts := RunOptions{
			Image:   "debian:stable-slim",
			Command: []string{"false"},
		}

		result, err := engine.Run(ctx, opts)
		// Run returns nil error but captures exit code in result
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ExitCode != 42 {
			t.Errorf("expected exit code 42, got %d", result.ExitCode)
		}
	})

	t.Run("remove failure", func(t *testing.T) {
		t.Parallel()

		recorder := NewMockCommandRecorder()
		recorder.Stderr = "Error: No such container"
		recorder.ExitCode = 1
		engine := newTestPodmanEngine(t, recorder)

		err := engine.Remove(ctx, "nonexistent-container", false)
		if err == nil {
			t.Fatal("expected error for failed remove")
		}
		if !strings.Contains(err.Error(), "failed") {
			t.Errorf("error should indicate failure, got: %v", err)
		}
	})

	t.Run("remove image failure", func(t *testing.T) {
		t.Parallel()

		recorder := NewMockCommandRecorder()
		recorder.Stderr = "Error: image is being used"
		recorder.ExitCode = 1
		engine := newTestPodmanEngine(t, recorder)

		err := engine.RemoveImage(ctx, "image-in-use:latest", false)
		if err == nil {
			t.Fatal("expected error for failed image removal")
		}
		if !strings.Contains(err.Error(), "failed") {
			t.Errorf("error should indicate failure, got: %v", err)
		}
	})

	t.Run("version failure", func(t *testing.T) {
		t.Parallel()

		recorder := NewMockCommandRecorder()
		recorder.Stderr = "Cannot connect to Podman socket"
		recorder.ExitCode = 1
		engine := newTestPodmanEngine(t, recorder)

		_, err := engine.Version(ctx)
		if err == nil {
			t.Fatal("expected error when Podman not available")
		}
		if !strings.Contains(err.Error(), "failed to get podman version") {
			t.Errorf("error should indicate version failure, got: %v", err)
		}
	})

	t.Run("exec failure", func(t *testing.T) {
		t.Parallel()

		recorder := NewMockCommandRecorder()
		recorder.Stderr = "Error: container is not running"
		recorder.ExitCode = 1
		engine := newTestPodmanEngine(t, recorder)

		result, err := engine.Exec(ctx, "stopped-container", []string{"ls"}, RunOptions{})
		// Exec returns nil error but captures exit code
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ExitCode == 0 {
			t.Error("expected non-zero exit code for stopped container")
		}
	})
}

// TestPodmanEngine_Version_Arguments verifies Podman Version() uses different format.
func TestPodmanEngine_Version_Arguments(t *testing.T) {
	t.Parallel()

	recorder := NewMockCommandRecorder()
	recorder.Stdout = "5.0.0"
	engine := newTestPodmanEngine(t, recorder)
	ctx := context.Background()

	version, err := engine.Version(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	recorder.AssertInvocationCount(t, 1)
	recorder.AssertFirstArg(t, "version")
	// Podman uses {{.Version}} instead of {{.Server.Version}}
	recorder.AssertArgsContain(t, "{{.Version}}")

	if version != "5.0.0" {
		t.Errorf("expected version '5.0.0', got %q", version)
	}
}

// TestPodmanEngine_Remove_Arguments verifies Podman Remove() constructs correct arguments.
func TestPodmanEngine_Remove_Arguments(t *testing.T) {
	t.Parallel()

	t.Run("basic remove", func(t *testing.T) {
		t.Parallel()
		recorder := NewMockCommandRecorder()
		engine := newTestPodmanEngine(t, recorder)
		ctx := context.Background()

		err := engine.Remove(ctx, "container123", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertInvocationCount(t, 1)
		recorder.AssertCommandName(t, "/usr/bin/podman")
		recorder.AssertFirstArg(t, "rm")
		recorder.AssertArgsContain(t, "container123")
		recorder.AssertArgsNotContain(t, "-f")
	})

	t.Run("force remove", func(t *testing.T) {
		t.Parallel()
		recorder := NewMockCommandRecorder()
		engine := newTestPodmanEngine(t, recorder)
		ctx := context.Background()

		err := engine.Remove(ctx, "container456", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "-f")
		recorder.AssertArgsContain(t, "container456")
	})
}

// TestPodmanEngine_RemoveImage_Arguments verifies Podman RemoveImage() constructs correct arguments.
func TestPodmanEngine_RemoveImage_Arguments(t *testing.T) {
	t.Parallel()

	t.Run("basic remove image", func(t *testing.T) {
		t.Parallel()
		recorder := NewMockCommandRecorder()
		engine := newTestPodmanEngine(t, recorder)
		ctx := context.Background()

		err := engine.RemoveImage(ctx, "myimage:latest", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertInvocationCount(t, 1)
		recorder.AssertCommandName(t, "/usr/bin/podman")
		recorder.AssertFirstArg(t, "rmi")
		recorder.AssertArgsContain(t, "myimage:latest")
		recorder.AssertArgsNotContain(t, "-f")
	})

	t.Run("force remove image", func(t *testing.T) {
		t.Parallel()
		recorder := NewMockCommandRecorder()
		engine := newTestPodmanEngine(t, recorder)
		ctx := context.Background()

		err := engine.RemoveImage(ctx, "myimage:v2", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "-f")
		recorder.AssertArgsContain(t, "myimage:v2")
	})
}

// TestPodmanEngine_Exec_Arguments verifies Exec() constructs correct arguments.
func TestPodmanEngine_Exec_Arguments(t *testing.T) {
	t.Parallel()

	recorder := NewMockCommandRecorder()
	engine := newTestPodmanEngine(t, recorder)
	ctx := context.Background()

	t.Run("basic exec", func(t *testing.T) {
		recorder.Reset()

		result, err := engine.Exec(ctx, "container123", []string{"ls", "-la"}, RunOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertInvocationCount(t, 1)
		recorder.AssertCommandName(t, "/usr/bin/podman")
		recorder.AssertFirstArg(t, "exec")
		recorder.AssertArgsContain(t, "container123")
		recorder.AssertArgsContain(t, "ls")
		recorder.AssertArgsContain(t, "-la")

		if result.ContainerID != "container123" {
			t.Errorf("expected ContainerID %q, got %q", "container123", result.ContainerID)
		}
	})

	t.Run("with interactive and tty", func(t *testing.T) {
		recorder.Reset()

		_, err := engine.Exec(ctx, "container456", []string{"bash"}, RunOptions{
			Interactive: true,
			TTY:         true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "-i")
		recorder.AssertArgsContain(t, "-t")
		recorder.AssertArgsContain(t, "container456")
		recorder.AssertArgsContain(t, "bash")
	})

	t.Run("with workdir and env", func(t *testing.T) {
		recorder.Reset()

		_, err := engine.Exec(ctx, "container789", []string{"./build.sh"}, RunOptions{
			WorkDir: "/app",
			Env: map[string]string{
				"BUILD_MODE": "release",
				"DEBUG":      "false",
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "-w")
		recorder.AssertArgsContain(t, "/app")
		recorder.AssertArgsContain(t, "-e")

		args := strings.Join(recorder.LastArgs(), " ")
		if !strings.Contains(args, "BUILD_MODE=release") {
			t.Errorf("expected BUILD_MODE env var, got: %v", recorder.LastArgs())
		}
		if !strings.Contains(args, "DEBUG=false") {
			t.Errorf("expected DEBUG env var, got: %v", recorder.LastArgs())
		}
	})

	t.Run("exit code capture", func(t *testing.T) {
		t.Parallel()

		// Use a fresh recorder with non-zero exit code
		recorderWithExit := NewMockCommandRecorder()
		recorderWithExit.Stderr = "command failed"
		recorderWithExit.ExitCode = 42
		engineWithExit := newTestPodmanEngine(t, recorderWithExit)

		result, err := engineWithExit.Exec(ctx, "failing-container", []string{"false"}, RunOptions{})
		// Exec returns nil error but captures exit code in result
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ExitCode != 42 {
			t.Errorf("expected exit code 42, got %d", result.ExitCode)
		}
	})
}

// TestPodmanEngine_InspectImage_Arguments verifies InspectImage() constructs correct arguments.
func TestPodmanEngine_InspectImage_Arguments(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("basic inspect", func(t *testing.T) {
		t.Parallel()

		recorder := NewMockCommandRecorder()
		recorder.Stdout = `{"Id": "sha256:abc123"}`
		engine := newTestPodmanEngine(t, recorder)

		output, err := engine.InspectImage(ctx, "debian:stable-slim")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertInvocationCount(t, 1)
		recorder.AssertFirstArg(t, "image")
		recorder.AssertArgsContain(t, "inspect")
		recorder.AssertArgsContain(t, "debian:stable-slim")

		if !strings.Contains(output, "sha256:abc123") {
			t.Errorf("expected output to contain image ID, got %q", output)
		}
	})

	t.Run("with registry", func(t *testing.T) {
		t.Parallel()

		recorder := NewMockCommandRecorder()
		engine := newTestPodmanEngine(t, recorder)

		_, err := engine.InspectImage(ctx, "ghcr.io/invowk/invowk:v1.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "ghcr.io/invowk/invowk:v1.0.0")
	})

	t.Run("image not found error", func(t *testing.T) {
		t.Parallel()

		recorder := NewMockCommandRecorder()
		recorder.Stderr = "Error: No such image"
		recorder.ExitCode = 1
		engine := newTestPodmanEngine(t, recorder)

		_, err := engine.InspectImage(ctx, "nonexistent:latest")
		if err == nil {
			t.Fatal("expected error for nonexistent image")
		}
	})
}

// =============================================================================
// SELinux Volume Labeling Tests
// =============================================================================

// newTestPodmanEngineWithSELinux creates a PodmanEngine with injectable SELinux check.
func newTestPodmanEngineWithSELinux(t *testing.T, recorder *MockCommandRecorder, selinuxEnabled bool) *PodmanEngine {
	t.Helper()
	return NewPodmanEngineWithSELinuxCheck(
		func() bool { return selinuxEnabled },
		WithExecCommand(recorder.ContextCommandFunc(t)),
	)
}

// TestPodmanEngine_SELinuxVolumeLabeling verifies SELinux label handling for volumes.
func TestPodmanEngine_SELinuxVolumeLabeling(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name           string
		selinuxEnabled bool
		volume         string
		expectedInArgs string
	}{
		// SELinux enabled cases
		{
			name:           "SELinux enabled - adds :z to simple volume",
			selinuxEnabled: true,
			volume:         "/host:/container",
			expectedInArgs: "/host:/container:z",
		},
		{
			name:           "SELinux enabled - preserves existing :z",
			selinuxEnabled: true,
			volume:         "/host:/container:z",
			expectedInArgs: "/host:/container:z",
		},
		{
			name:           "SELinux enabled - preserves existing :Z",
			selinuxEnabled: true,
			volume:         "/host:/container:Z",
			expectedInArgs: "/host:/container:Z",
		},
		{
			name:           "SELinux enabled - appends z to ro",
			selinuxEnabled: true,
			volume:         "/host:/container:ro",
			expectedInArgs: "/host:/container:ro,z",
		},
		{
			name:           "SELinux enabled - appends z to rw",
			selinuxEnabled: true,
			volume:         "/host:/container:rw",
			expectedInArgs: "/host:/container:rw,z",
		},
		{
			name:           "SELinux enabled - handles multiple options",
			selinuxEnabled: true,
			volume:         "/host:/container:ro,exec",
			expectedInArgs: "/host:/container:ro,exec,z",
		},
		{
			name:           "SELinux enabled - does not double-add z",
			selinuxEnabled: true,
			volume:         "/host:/container:ro,z",
			expectedInArgs: "/host:/container:ro,z",
		},
		// SELinux disabled cases
		{
			name:           "SELinux disabled - no modification",
			selinuxEnabled: false,
			volume:         "/host:/container",
			expectedInArgs: "/host:/container",
		},
		{
			name:           "SELinux disabled - preserves existing options",
			selinuxEnabled: false,
			volume:         "/host:/container:ro",
			expectedInArgs: "/host:/container:ro",
		},
		{
			name:           "SELinux disabled - preserves existing z label",
			selinuxEnabled: false,
			volume:         "/host:/container:z",
			expectedInArgs: "/host:/container:z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := NewMockCommandRecorder()
			engine := newTestPodmanEngineWithSELinux(t, recorder, tt.selinuxEnabled)

			opts := RunOptions{
				Image:   "debian:stable-slim",
				Volumes: []invowkfile.VolumeMountSpec{invowkfile.VolumeMountSpec(tt.volume)},
			}

			_, err := engine.Run(ctx, opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			recorder.AssertArgsContain(t, tt.expectedInArgs)
		})
	}
}

// TestAddSELinuxLabelWithCheck_EdgeCases tests edge cases in SELinux label handling.
func TestAddSELinuxLabelWithCheck_EdgeCases(t *testing.T) {
	t.Parallel()

	selinuxEnabled := func() bool { return true }
	selinuxDisabled := func() bool { return false }

	tests := []struct {
		name         string
		volume       invowkfile.VolumeMountSpec
		selinuxCheck SELinuxCheckFunc
		expected     string
	}{
		// Edge cases with SELinux enabled
		{
			name:         "host path only - no container path",
			volume:       "/host",
			selinuxCheck: selinuxEnabled,
			expected:     "/host", // Can't add label without container path
		},
		{
			name:         "empty string",
			volume:       "",
			selinuxCheck: selinuxEnabled,
			expected:     "",
		},
		{
			name:         "multiple colons in path",
			volume:       "/host:/container:/extra",
			selinuxCheck: selinuxEnabled,
			expected:     "/host:/container:/extra,z", // Extra treated as options
		},
		{
			name:         "named volume",
			volume:       "myvolume:/container",
			selinuxCheck: selinuxEnabled,
			expected:     "myvolume:/container:z",
		},
		{
			name:         "named volume with options",
			volume:       "myvolume:/container:ro",
			selinuxCheck: selinuxEnabled,
			expected:     "myvolume:/container:ro,z",
		},
		// Edge cases with SELinux disabled
		{
			name:         "host path only - SELinux disabled",
			volume:       "/host",
			selinuxCheck: selinuxDisabled,
			expected:     "/host",
		},
		{
			name:         "empty string - SELinux disabled",
			volume:       "",
			selinuxCheck: selinuxDisabled,
			expected:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := addSELinuxLabelWithCheck(tt.volume, tt.selinuxCheck)
			if got != tt.expected {
				t.Errorf("addSELinuxLabelWithCheck(%q) = %q, want %q", tt.volume, got, tt.expected)
			}
		})
	}
}

// TestMakeSELinuxLabelAdder verifies the factory function creates correct formatters.
func TestMakeSELinuxLabelAdder(t *testing.T) {
	t.Parallel()

	t.Run("returns function that adds labels when SELinux enabled", func(t *testing.T) {
		t.Parallel()

		formatter := makeSELinuxLabelAdder(func() bool { return true })
		result := formatter(invowkfile.VolumeMountSpec("/host:/container"))
		if result != "/host:/container:z" {
			t.Errorf("formatter returned %q, want %q", result, "/host:/container:z")
		}
	})

	t.Run("returns function that preserves volume when SELinux disabled", func(t *testing.T) {
		t.Parallel()

		formatter := makeSELinuxLabelAdder(func() bool { return false })
		result := formatter(invowkfile.VolumeMountSpec("/host:/container"))
		if result != "/host:/container" {
			t.Errorf("formatter returned %q, want %q", result, "/host:/container")
		}
	})
}
