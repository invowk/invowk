// SPDX-License-Identifier: MPL-2.0

package container

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const dockerInspectSubcommand = "inspect"

// =============================================================================
// Docker Engine Mock Tests (T069, T070, T071, T072)
// =============================================================================

// newTestDockerEngine creates a DockerEngine for testing with the mock recorder.
func newTestDockerEngine(t *testing.T, recorder *MockCommandRecorder) *DockerEngine {
	t.Helper()
	return &DockerEngine{
		BaseCLIEngine: NewBaseCLIEngine("/usr/bin/docker", WithExecCommand(recorder.ContextCommandFunc(t))),
	}
}

// TestDockerEngine_Build_Arguments verifies Build() constructs correct arguments.
func TestDockerEngine_Build_Arguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opts     BuildOptions
		wantArgs []string
	}{
		{name: "basic build", opts: BuildOptions{ContextDir: "/tmp/build", Tag: "myimage:latest"}, wantArgs: []string{"-t", "myimage:latest", "/tmp/build"}},
		{name: "with dockerfile", opts: BuildOptions{ContextDir: "/tmp/build", Dockerfile: "Dockerfile.custom", Tag: "test:v1"}, wantArgs: []string{"-f", filepath.Join(string(filepath.Separator), "tmp", "build", "Dockerfile.custom")}},
		{name: "with no-cache", opts: BuildOptions{ContextDir: "/tmp/build", Tag: "test:v1", NoCache: true}, wantArgs: []string{"--no-cache"}},
		{name: "with build args", opts: BuildOptions{ContextDir: "/tmp/build", Tag: "test:v1", BuildArgs: map[string]string{"VERSION": "1.0.0", "DEBUG": "true"}}, wantArgs: []string{"--build-arg", "VERSION=1.0.0", "DEBUG=true"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := NewMockCommandRecorder()
			engine := newTestDockerEngine(t, recorder)
			if err := engine.Build(t.Context(), tt.opts); err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			recorder.AssertInvocationCount(t, 1)
			recorder.AssertCommandName(t, "/usr/bin/docker")
			recorder.AssertFirstArg(t, "build")
			recorder.AssertArgsContainAll(t, tt.wantArgs)
		})
	}
}

// TestDockerEngine_Run_Arguments verifies Run() constructs correct arguments.
func TestDockerEngine_Run_Arguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		opts     RunOptions
		wantArgs []string
	}{
		{name: "basic run", opts: RunOptions{Image: "debian:stable-slim", Command: []string{"echo", "hello"}}, wantArgs: []string{"debian:stable-slim", "echo", "hello"}},
		{name: "with remove flag", opts: RunOptions{Image: "debian:stable-slim", Command: []string{"true"}, Remove: true}, wantArgs: []string{"--rm"}},
		{name: "with container name", opts: RunOptions{Image: "debian:stable-slim", Command: []string{"true"}, Name: "my-container"}, wantArgs: []string{"--name", "my-container"}},
		{name: "with workdir", opts: RunOptions{Image: "debian:stable-slim", Command: []string{"pwd"}, WorkDir: "/app"}, wantArgs: []string{"-w", "/app"}},
		{name: "with interactive and tty", opts: RunOptions{Image: "debian:stable-slim", Command: []string{"bash"}, Interactive: true, TTY: true}, wantArgs: []string{"-i", "-t"}},
		{name: "with environment variables", opts: RunOptions{Image: "debian:stable-slim", Command: []string{"env"}, Env: map[string]string{"FOO": "bar", "BAZ": "qux"}}, wantArgs: []string{"-e", "FOO=bar", "BAZ=qux"}},
		{name: "with volumes", opts: RunOptions{Image: "debian:stable-slim", Command: []string{"ls"}, Volumes: []VolumeMountSpec{"/host/path:/container/path", "/data:/data:ro"}}, wantArgs: []string{"-v", "/host/path:/container/path", "/data:/data:ro"}},
		{name: "with ports", opts: RunOptions{Image: "debian:stable-slim", Command: []string{"true"}, Ports: []PortMappingSpec{"8080:80", "443:443"}}, wantArgs: []string{"-p", "8080:80", "443:443"}},
		{name: "with extra hosts", opts: RunOptions{Image: "debian:stable-slim", Command: []string{"true"}, ExtraHosts: []HostMapping{"host.docker.internal:host-gateway"}}, wantArgs: []string{"--add-host", "host.docker.internal:host-gateway"}},
		{name: "full options", opts: RunOptions{
			Image:       "debian:stable-slim",
			Command:     []string{"./script.sh", "arg1", "arg2"},
			WorkDir:     "/workspace",
			Name:        "full-test",
			Remove:      true,
			Interactive: true,
			TTY:         true,
			Env:         map[string]string{"DEBUG": "1"},
			Volumes:     []VolumeMountSpec{"/src:/src"},
			Ports:       []PortMappingSpec{"3000:3000"},
			ExtraHosts:  []HostMapping{"db:192.168.1.100"},
		}, wantArgs: []string{
			"run", "--rm", "--name", "full-test", "-w", "/workspace",
			"-i", "-t", "-e", "DEBUG=1", "-v", "/src:/src", "-p", "3000:3000",
			"--add-host", "db:192.168.1.100", "debian:stable-slim",
			"./script.sh", "arg1", "arg2",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := NewMockCommandRecorder()
			engine := newTestDockerEngine(t, recorder)
			if _, err := engine.Run(t.Context(), tt.opts); err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			recorder.AssertInvocationCount(t, 1)
			recorder.AssertCommandName(t, "/usr/bin/docker")
			recorder.AssertFirstArg(t, "run")
			recorder.AssertArgsContainAll(t, tt.wantArgs)
		})
	}
}

// TestDockerEngine_ImageExists_Arguments verifies ImageExists() constructs correct arguments.
func TestDockerEngine_ImageExists_Arguments(t *testing.T) {
	t.Parallel()

	t.Run("image exists check", func(t *testing.T) {
		t.Parallel()
		recorder := NewMockCommandRecorder()
		engine := newTestDockerEngine(t, recorder)
		ctx := t.Context()

		exists, err := engine.ImageExists(ctx, "myimage:latest")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !exists {
			t.Error("expected image to exist (mock returns success)")
		}

		recorder.AssertInvocationCount(t, 1)
		recorder.AssertCommandName(t, "/usr/bin/docker")
		recorder.AssertFirstArg(t, "image")
		recorder.AssertArgsContain(t, dockerInspectSubcommand)
		recorder.AssertArgsContain(t, "myimage:latest")
	})

	t.Run("image with registry", func(t *testing.T) {
		t.Parallel()
		recorder := NewMockCommandRecorder()
		engine := newTestDockerEngine(t, recorder)
		ctx := t.Context()

		_, err := engine.ImageExists(ctx, "ghcr.io/invowk/invowk:v1.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "ghcr.io/invowk/invowk:v1.0.0")
	})
}

// TestDockerEngine_ErrorPaths verifies error handling (T072).
func TestDockerEngine_ErrorPaths(t *testing.T) {
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
		{name: "version failure", operation: "version", stderr: "Cannot connect to Docker daemon", exitCode: 1, wantExitError: true},
		{name: "exec failure", operation: "exec", stderr: "Error: container is not running", exitCode: 1, checkResultExit: true, wantResultExit: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := NewMockCommandRecorder()
			recorder.Stderr, recorder.ExitCode = tt.stderr, tt.exitCode
			engine := newTestDockerEngine(t, recorder)
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

// TestDockerEngine_Remove_Arguments verifies Remove() constructs correct arguments.
func TestDockerEngine_Remove_Arguments(t *testing.T) {
	t.Parallel()

	t.Run("basic remove", func(t *testing.T) {
		t.Parallel()
		recorder := NewMockCommandRecorder()
		engine := newTestDockerEngine(t, recorder)
		ctx := t.Context()

		err := engine.Remove(ctx, "container123", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertInvocationCount(t, 1)
		recorder.AssertFirstArg(t, "rm")
		recorder.AssertArgsContain(t, "container123")
		recorder.AssertArgsNotContain(t, "-f")
	})

	t.Run("force remove", func(t *testing.T) {
		t.Parallel()
		recorder := NewMockCommandRecorder()
		engine := newTestDockerEngine(t, recorder)
		ctx := t.Context()

		err := engine.Remove(ctx, "container123", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "-f")
		recorder.AssertArgsContain(t, "container123")
	})
}

// TestDockerEngine_RemoveImage_Arguments verifies RemoveImage() constructs correct arguments.
func TestDockerEngine_RemoveImage_Arguments(t *testing.T) {
	t.Parallel()

	t.Run("basic remove image", func(t *testing.T) {
		t.Parallel()
		recorder := NewMockCommandRecorder()
		engine := newTestDockerEngine(t, recorder)
		ctx := t.Context()

		err := engine.RemoveImage(ctx, "myimage:latest", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertInvocationCount(t, 1)
		recorder.AssertFirstArg(t, "rmi")
		recorder.AssertArgsContain(t, "myimage:latest")
		recorder.AssertArgsNotContain(t, "-f")
	})

	t.Run("force remove image", func(t *testing.T) {
		t.Parallel()
		recorder := NewMockCommandRecorder()
		engine := newTestDockerEngine(t, recorder)
		ctx := t.Context()

		err := engine.RemoveImage(ctx, "myimage:latest", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "-f")
	})
}

// TestDockerEngine_Version_Arguments verifies Version() constructs correct arguments.
func TestDockerEngine_Version_Arguments(t *testing.T) {
	t.Parallel()

	recorder := NewMockCommandRecorder()
	recorder.Stdout = "24.0.7"
	engine := newTestDockerEngine(t, recorder)
	ctx := t.Context()

	version, err := engine.Version(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	recorder.AssertInvocationCount(t, 1)
	recorder.AssertFirstArg(t, "version")
	recorder.AssertArgsContain(t, "--format")

	if version != "24.0.7" {
		t.Errorf("expected version '24.0.7', got %q", version)
	}
}

// TestDockerEngine_Exec_Arguments verifies Exec() constructs correct arguments.
func TestDockerEngine_Exec_Arguments(t *testing.T) {
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
			recorder.Stderr = tt.stderr
			recorder.ExitCode = tt.exitCode
			engine := newTestDockerEngine(t, recorder)
			result, err := engine.Exec(t.Context(), tt.container, tt.command, tt.opts)
			if err != nil {
				t.Fatalf("Exec() error = %v", err)
			}
			recorder.AssertInvocationCount(t, 1)
			recorder.AssertCommandName(t, "/usr/bin/docker")
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

// TestDockerEngine_InspectImage_Arguments verifies InspectImage() constructs correct arguments.
func TestDockerEngine_InspectImage_Arguments(t *testing.T) {
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
		{name: "basic inspect", image: "debian:stable-slim", stdout: `{"Id": "sha256:abc123"}`, wantArgs: []string{dockerInspectSubcommand, "debian:stable-slim"}, wantOutput: "sha256:abc123"},
		{name: "with registry", image: "ghcr.io/invowk/invowk:v1.0.0", wantArgs: []string{"ghcr.io/invowk/invowk:v1.0.0"}},
		{name: "image not found error", image: "nonexistent:latest", stderr: "Error: No such image", exitCode: 1, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := NewMockCommandRecorder()
			recorder.Stdout, recorder.Stderr, recorder.ExitCode = tt.stdout, tt.stderr, tt.exitCode
			engine := newTestDockerEngine(t, recorder)
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
