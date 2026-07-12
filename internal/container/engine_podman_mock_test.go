// SPDX-License-Identifier: MPL-2.0

package container

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
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
		ctx := t.Context()

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
		ctx := t.Context()

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
		ctx := t.Context()

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
		ctx := t.Context()

		opts := RunOptions{
			Image:       "debian:stable-slim",
			Command:     []string{"bash", "-c", "echo test"},
			WorkDir:     "/app",
			Name:        "podman-test",
			Remove:      true,
			Interactive: true,
			TTY:         true,
			Env:         map[string]string{"VAR": "value"},
			Volumes:     []VolumeMountSpec{"/src:/src"},
			Ports:       []PortMappingSpec{"8080:80"},
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
		ctx := t.Context()

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

	ctx := t.Context()

	tests := []struct {
		name               string
		operation          string
		stderr             string
		exitCode           int
		wantOperationError bool
		wantExitError      bool
		checkExists        bool
		wantExists         bool
		checkResultExit    bool
		wantResultExit     int
	}{
		{name: "build failure", operation: "build", stderr: "Error: build failed", exitCode: 1, wantOperationError: true},
		{name: "image not found", operation: "imageExists", stderr: "Error: No such image", exitCode: 1, checkExists: true},
		{name: "run with exit code", operation: "run", stderr: "command failed", exitCode: 42, checkResultExit: true, wantResultExit: 42},
		{name: "remove failure", operation: "remove", stderr: "Error: No such container", exitCode: 1, wantExitError: true},
		{name: "remove image failure", operation: "removeImage", stderr: "Error: image is being used", exitCode: 1, wantExitError: true},
		{name: "version failure", operation: "version", stderr: "Cannot connect to Podman socket", exitCode: 1, wantExitError: true},
		{name: "exec failure", operation: "exec", stderr: "Error: container is not running", exitCode: 1, checkResultExit: true, wantResultExit: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := NewMockCommandRecorder()
			recorder.Stderr, recorder.ExitCode = tt.stderr, tt.exitCode
			engine := newTestPodmanEngine(t, recorder)
			var err error
			var exists bool
			var result *RunResult
			switch tt.operation {
			case "build":
				err = engine.Build(ctx, BuildOptions{ContextDir: "/tmp/build", Tag: "test:v1"})
			case "imageExists":
				exists, err = engine.ImageExists(ctx, "nonexistent:latest")
			case "run":
				result, err = engine.Run(ctx, RunOptions{Image: "debian:stable-slim", Command: []string{"false"}})
			case "remove":
				err = engine.Remove(ctx, "nonexistent-container", false)
			case "removeImage":
				err = engine.RemoveImage(ctx, "image-in-use:latest", false)
			case "version":
				_, err = engine.Version(ctx)
			case "exec":
				result, err = engine.Exec(ctx, "stopped-container", []string{"ls"}, RunOptions{})
			default:
				t.Fatalf("unknown operation %q", tt.operation)
			}
			switch {
			case tt.wantOperationError:
				if _, ok := errors.AsType[*OperationError](err); !ok {
					t.Fatalf("error = %T %v, want *OperationError", err, err)
				}
			case tt.wantExitError:
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) {
					t.Fatalf("error = %T %v, want wrapped *exec.ExitError", err, err)
				}
			case err != nil:
				t.Fatalf("operation error = %v", err)
			}
			if tt.checkExists && exists != tt.wantExists {
				t.Errorf("ImageExists() = %t, want %t", exists, tt.wantExists)
			}
			if tt.checkResultExit && int(result.ExitCode) != tt.wantResultExit {
				t.Errorf("result.ExitCode = %d, want %d", result.ExitCode, tt.wantResultExit)
			}
		})
	}
}

// TestPodmanEngine_Version_Arguments verifies Podman Version() uses different format.
func TestPodmanEngine_Version_Arguments(t *testing.T) {
	t.Parallel()

	recorder := NewMockCommandRecorder()
	recorder.Stdout = "5.0.0"
	engine := newTestPodmanEngine(t, recorder)
	ctx := t.Context()

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
		ctx := t.Context()

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
		ctx := t.Context()

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
		ctx := t.Context()

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
		ctx := t.Context()

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

	tests := []struct {
		name            string
		container       ContainerID
		command         []string
		opts            RunOptions
		stderr          string
		exitCode        int
		wantArgs        []string
		wantContainerID ContainerID
		wantExitCode    int
	}{
		{name: "basic exec", container: "container123", command: []string{"ls", "-la"}, wantArgs: []string{"container123", "ls", "-la"}, wantContainerID: "container123"},
		{name: "with interactive and tty", container: "container456", command: []string{"bash"}, opts: RunOptions{Interactive: true, TTY: true}, wantArgs: []string{"-i", "-t", "container456", "bash"}},
		{name: "with workdir and env", container: "container789", command: []string{"./build.sh"}, opts: RunOptions{WorkDir: "/app", Env: map[string]string{"BUILD_MODE": "release", "DEBUG": "false"}}, wantArgs: []string{"-w", "/app", "-e", "BUILD_MODE=release", "DEBUG=false"}},
		{name: "exit code capture", container: "failing-container", command: []string{"false"}, stderr: "command failed", exitCode: 42, wantExitCode: 42},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := NewMockCommandRecorder()
			recorder.Stderr, recorder.ExitCode = tt.stderr, tt.exitCode
			engine := newTestPodmanEngine(t, recorder)
			result, err := engine.Exec(t.Context(), tt.container, tt.command, tt.opts)
			if err != nil {
				t.Fatalf("Exec() error = %v", err)
			}
			recorder.AssertInvocationCount(t, 1)
			recorder.AssertCommandName(t, "/usr/bin/podman")
			recorder.AssertFirstArg(t, "exec")
			recorder.AssertArgsContainAll(t, tt.wantArgs)
			if tt.wantContainerID != "" && result.ContainerID != tt.wantContainerID {
				t.Errorf("ContainerID = %q, want %q", result.ContainerID, tt.wantContainerID)
			}
			if int(result.ExitCode) != tt.wantExitCode {
				t.Errorf("ExitCode = %d, want %d", result.ExitCode, tt.wantExitCode)
			}
		})
	}
}

// TestPodmanEngine_InspectImage_Arguments verifies InspectImage() constructs correct arguments.
func TestPodmanEngine_InspectImage_Arguments(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	tests := []struct {
		name       string
		image      ImageTag
		stdout     string
		stderr     string
		exitCode   int
		wantArgs   []string
		wantOutput string
		wantErr    bool
	}{
		{name: "basic inspect", image: "debian:stable-slim", stdout: `{"Id": "sha256:abc123"}`, wantArgs: []string{"inspect", "debian:stable-slim"}, wantOutput: "sha256:abc123"},
		{name: "with registry", image: "ghcr.io/invowk/invowk:v1.0.0", wantArgs: []string{"ghcr.io/invowk/invowk:v1.0.0"}},
		{name: "image not found error", image: "nonexistent:latest", stderr: "Error: No such image", exitCode: 1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := NewMockCommandRecorder()
			recorder.Stdout, recorder.Stderr, recorder.ExitCode = tt.stdout, tt.stderr, tt.exitCode
			engine := newTestPodmanEngine(t, recorder)
			output, err := engine.InspectImage(ctx, tt.image)
			if (err != nil) != tt.wantErr {
				t.Fatalf("InspectImage() error = %v, wantErr %t", err, tt.wantErr)
			}
			recorder.AssertInvocationCount(t, 1)
			recorder.AssertFirstArg(t, "image")
			recorder.AssertArgsContainAll(t, tt.wantArgs)
			if tt.wantOutput != "" && !strings.Contains(output, tt.wantOutput) {
				t.Errorf("InspectImage() output = %q, want text %q", output, tt.wantOutput)
			}
		})
	}
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

	ctx := t.Context()

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
				Volumes: []VolumeMountSpec{VolumeMountSpec(tt.volume)},
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
		volume       VolumeMountSpec
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
		result := formatter(VolumeMountSpec("/host:/container"))
		if result != "/host:/container:z" {
			t.Errorf("formatter returned %q, want %q", result, "/host:/container:z")
		}
	})

	t.Run("returns function that preserves volume when SELinux disabled", func(t *testing.T) {
		t.Parallel()

		formatter := makeSELinuxLabelAdder(func() bool { return false })
		result := formatter(VolumeMountSpec("/host:/container"))
		if result != "/host:/container" {
			t.Errorf("formatter returned %q, want %q", result, "/host:/container")
		}
	})
}
