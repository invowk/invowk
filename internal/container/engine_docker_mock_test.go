// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

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
	recorder, cleanup := withMockExecCommand(t)
	defer cleanup()

	engine := newTestDockerEngine(t, recorder)
	ctx := context.Background()

	t.Run("basic build", func(t *testing.T) {
		recorder.Reset()
		opts := BuildOptions{
			ContextDir: "/tmp/build",
			Tag:        "myimage:latest",
		}

		err := engine.Build(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertInvocationCount(t, 1)
		recorder.AssertCommandName(t, "/usr/bin/docker")
		recorder.AssertFirstArg(t, "build")
		recorder.AssertArgsContain(t, "-t")
		recorder.AssertArgsContain(t, "myimage:latest")
		recorder.AssertArgsContain(t, "/tmp/build")
	})

	t.Run("with dockerfile", func(t *testing.T) {
		recorder.Reset()
		opts := BuildOptions{
			ContextDir: "/tmp/build",
			Dockerfile: "Dockerfile.custom",
			Tag:        "test:v1",
		}

		err := engine.Build(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "-f")
		// Dockerfile path should be joined with context dir (use filepath.Join for cross-platform)
		//nolint:gocritic // filepathJoin: testing production code that joins dir path with filename
		recorder.AssertArgsContain(t, filepath.Join("/tmp/build", "Dockerfile.custom"))
	})

	t.Run("with no-cache", func(t *testing.T) {
		recorder.Reset()
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

	t.Run("with build args", func(t *testing.T) {
		recorder.Reset()
		opts := BuildOptions{
			ContextDir: "/tmp/build",
			Tag:        "test:v1",
			BuildArgs: map[string]string{
				"VERSION": "1.0.0",
				"DEBUG":   "true",
			},
		}

		err := engine.Build(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "--build-arg")
		// Note: map iteration order is not guaranteed, so we check both variations
		args := strings.Join(recorder.LastArgs(), " ")
		if !strings.Contains(args, "VERSION=1.0.0") {
			t.Errorf("expected VERSION build arg, got: %v", recorder.LastArgs())
		}
		if !strings.Contains(args, "DEBUG=true") {
			t.Errorf("expected DEBUG build arg, got: %v", recorder.LastArgs())
		}
	})
}

// TestDockerEngine_Run_Arguments verifies Run() constructs correct arguments.
func TestDockerEngine_Run_Arguments(t *testing.T) {
	recorder, cleanup := withMockExecCommand(t)
	defer cleanup()

	engine := newTestDockerEngine(t, recorder)
	ctx := context.Background()

	t.Run("basic run", func(t *testing.T) {
		recorder.Reset()
		opts := RunOptions{
			Image:   "debian:stable-slim",
			Command: []string{"echo", "hello"},
		}

		_, err := engine.Run(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertInvocationCount(t, 1)
		recorder.AssertCommandName(t, "/usr/bin/docker")
		recorder.AssertFirstArg(t, "run")
		recorder.AssertArgsContain(t, "debian:stable-slim")
		recorder.AssertArgsContain(t, "echo")
		recorder.AssertArgsContain(t, "hello")
	})

	t.Run("with remove flag", func(t *testing.T) {
		recorder.Reset()
		opts := RunOptions{
			Image:   "debian:stable-slim",
			Command: []string{"true"},
			Remove:  true,
		}

		_, err := engine.Run(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "--rm")
	})

	t.Run("with container name", func(t *testing.T) {
		recorder.Reset()
		opts := RunOptions{
			Image:   "debian:stable-slim",
			Command: []string{"true"},
			Name:    "my-container",
		}

		_, err := engine.Run(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "--name")
		recorder.AssertArgsContain(t, "my-container")
	})

	t.Run("with workdir", func(t *testing.T) {
		recorder.Reset()
		opts := RunOptions{
			Image:   "debian:stable-slim",
			Command: []string{"pwd"},
			WorkDir: "/app",
		}

		_, err := engine.Run(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "-w")
		recorder.AssertArgsContain(t, "/app")
	})

	t.Run("with interactive and tty", func(t *testing.T) {
		recorder.Reset()
		opts := RunOptions{
			Image:       "debian:stable-slim",
			Command:     []string{"bash"},
			Interactive: true,
			TTY:         true,
		}

		_, err := engine.Run(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "-i")
		recorder.AssertArgsContain(t, "-t")
	})

	t.Run("with environment variables", func(t *testing.T) {
		recorder.Reset()
		opts := RunOptions{
			Image:   "debian:stable-slim",
			Command: []string{"env"},
			Env: map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			},
		}

		_, err := engine.Run(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "-e")
		args := strings.Join(recorder.LastArgs(), " ")
		if !strings.Contains(args, "FOO=bar") {
			t.Errorf("expected FOO=bar env var, got: %v", recorder.LastArgs())
		}
		if !strings.Contains(args, "BAZ=qux") {
			t.Errorf("expected BAZ=qux env var, got: %v", recorder.LastArgs())
		}
	})

	t.Run("with volumes", func(t *testing.T) {
		recorder.Reset()
		opts := RunOptions{
			Image:   "debian:stable-slim",
			Command: []string{"ls"},
			Volumes: []string{"/host/path:/container/path", "/data:/data:ro"},
		}

		_, err := engine.Run(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "-v")
		recorder.AssertArgsContain(t, "/host/path:/container/path")
		recorder.AssertArgsContain(t, "/data:/data:ro")
	})

	t.Run("with ports", func(t *testing.T) {
		recorder.Reset()
		opts := RunOptions{
			Image:   "debian:stable-slim",
			Command: []string{"true"},
			Ports:   []string{"8080:80", "443:443"},
		}

		_, err := engine.Run(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "-p")
		recorder.AssertArgsContain(t, "8080:80")
		recorder.AssertArgsContain(t, "443:443")
	})

	t.Run("with extra hosts", func(t *testing.T) {
		recorder.Reset()
		opts := RunOptions{
			Image:      "debian:stable-slim",
			Command:    []string{"true"},
			ExtraHosts: []string{"host.docker.internal:host-gateway"},
		}

		_, err := engine.Run(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "--add-host")
		recorder.AssertArgsContain(t, "host.docker.internal:host-gateway")
	})

	t.Run("full options", func(t *testing.T) {
		recorder.Reset()
		opts := RunOptions{
			Image:       "debian:stable-slim",
			Command:     []string{"./script.sh", "arg1", "arg2"},
			WorkDir:     "/workspace",
			Name:        "full-test",
			Remove:      true,
			Interactive: true,
			TTY:         true,
			Env:         map[string]string{"DEBUG": "1"},
			Volumes:     []string{"/src:/src"},
			Ports:       []string{"3000:3000"},
			ExtraHosts:  []string{"db:192.168.1.100"},
		}

		_, err := engine.Run(ctx, opts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := []string{
			"run", "--rm", "--name", "full-test", "-w", "/workspace",
			"-i", "-t", "-e", "DEBUG=1", "-v", "/src:/src", "-p", "3000:3000",
			"--add-host", "db:192.168.1.100", "debian:stable-slim",
			"./script.sh", "arg1", "arg2",
		}
		recorder.AssertArgsContainAll(t, expected)
	})
}

// TestDockerEngine_ImageExists_Arguments verifies ImageExists() constructs correct arguments.
func TestDockerEngine_ImageExists_Arguments(t *testing.T) {
	recorder, cleanup := withMockExecCommand(t)
	defer cleanup()

	engine := newTestDockerEngine(t, recorder)
	ctx := context.Background()

	t.Run("image exists check", func(t *testing.T) {
		recorder.Reset()

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
		recorder.AssertArgsContain(t, "inspect")
		recorder.AssertArgsContain(t, "myimage:latest")
	})

	t.Run("image with registry", func(t *testing.T) {
		recorder.Reset()

		_, err := engine.ImageExists(ctx, "ghcr.io/invowk/invowk:v1.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "ghcr.io/invowk/invowk:v1.0.0")
	})
}

// TestDockerEngine_ErrorPaths verifies error handling (T072).
func TestDockerEngine_ErrorPaths(t *testing.T) {
	ctx := context.Background()

	t.Run("build failure", func(t *testing.T) {
		recorder, cleanup := withMockExecCommandOutput(t, "", "Error: build failed", 1)
		defer cleanup()
		engine := newTestDockerEngine(t, recorder)

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
		recorder, cleanup := withMockExecCommandOutput(t, "", "Error: No such image", 1)
		defer cleanup()
		engine := newTestDockerEngine(t, recorder)

		exists, err := engine.ImageExists(ctx, "nonexistent:latest")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// ImageExists returns false for non-existent images, not an error
		if exists {
			t.Error("expected image to not exist")
		}
	})

	t.Run("run with exit code", func(t *testing.T) {
		recorder, cleanup := withMockExecCommandOutput(t, "", "command failed", 42)
		defer cleanup()
		engine := newTestDockerEngine(t, recorder)

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
		recorder, cleanup := withMockExecCommandOutput(t, "", "Error: No such container", 1)
		defer cleanup()
		engine := newTestDockerEngine(t, recorder)

		err := engine.Remove(ctx, "nonexistent-container", false)
		if err == nil {
			t.Fatal("expected error for failed remove")
		}
		if !strings.Contains(err.Error(), "failed") {
			t.Errorf("error should indicate failure, got: %v", err)
		}
	})

	t.Run("remove image failure", func(t *testing.T) {
		recorder, cleanup := withMockExecCommandOutput(t, "", "Error: image is being used", 1)
		defer cleanup()
		engine := newTestDockerEngine(t, recorder)

		err := engine.RemoveImage(ctx, "image-in-use:latest", false)
		if err == nil {
			t.Fatal("expected error for failed image removal")
		}
		if !strings.Contains(err.Error(), "failed") {
			t.Errorf("error should indicate failure, got: %v", err)
		}
	})

	t.Run("version failure", func(t *testing.T) {
		recorder, cleanup := withMockExecCommandOutput(t, "", "Cannot connect to Docker daemon", 1)
		defer cleanup()
		engine := newTestDockerEngine(t, recorder)

		_, err := engine.Version(ctx)
		if err == nil {
			t.Fatal("expected error when daemon not available")
		}
		if !strings.Contains(err.Error(), "failed to get docker version") {
			t.Errorf("error should indicate version failure, got: %v", err)
		}
	})

	t.Run("exec failure", func(t *testing.T) {
		recorder, cleanup := withMockExecCommandOutput(t, "", "Error: container is not running", 1)
		defer cleanup()
		engine := newTestDockerEngine(t, recorder)

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

// TestDockerEngine_Remove_Arguments verifies Remove() constructs correct arguments.
func TestDockerEngine_Remove_Arguments(t *testing.T) {
	recorder, cleanup := withMockExecCommand(t)
	defer cleanup()

	engine := newTestDockerEngine(t, recorder)
	ctx := context.Background()

	t.Run("basic remove", func(t *testing.T) {
		recorder.Reset()

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
		recorder.Reset()

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
	recorder, cleanup := withMockExecCommand(t)
	defer cleanup()

	engine := newTestDockerEngine(t, recorder)
	ctx := context.Background()

	t.Run("basic remove image", func(t *testing.T) {
		recorder.Reset()

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
		recorder.Reset()

		err := engine.RemoveImage(ctx, "myimage:latest", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "-f")
	})
}

// TestDockerEngine_Version_Arguments verifies Version() constructs correct arguments.
func TestDockerEngine_Version_Arguments(t *testing.T) {
	recorder, cleanup := withMockExecCommandOutput(t, "24.0.7", "", 0)
	defer cleanup()

	engine := newTestDockerEngine(t, recorder)
	ctx := context.Background()

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
	recorder, cleanup := withMockExecCommand(t)
	defer cleanup()

	engine := newTestDockerEngine(t, recorder)
	ctx := context.Background()

	t.Run("basic exec", func(t *testing.T) {
		recorder.Reset()

		result, err := engine.Exec(ctx, "container123", []string{"ls", "-la"}, RunOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertInvocationCount(t, 1)
		recorder.AssertCommandName(t, "/usr/bin/docker")
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
		// Use a fresh recorder with non-zero exit code
		recorderWithExit, cleanupExit := withMockExecCommandOutput(t, "", "command failed", 42)
		defer cleanupExit()
		engineWithExit := newTestDockerEngine(t, recorderWithExit)

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

// TestDockerEngine_InspectImage_Arguments verifies InspectImage() constructs correct arguments.
func TestDockerEngine_InspectImage_Arguments(t *testing.T) {
	ctx := context.Background()

	t.Run("basic inspect", func(t *testing.T) {
		recorder, cleanup := withMockExecCommandOutput(t, `{"Id": "sha256:abc123"}`, "", 0)
		defer cleanup()
		engine := newTestDockerEngine(t, recorder)

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
		recorder, cleanup := withMockExecCommand(t)
		defer cleanup()
		engine := newTestDockerEngine(t, recorder)

		_, err := engine.InspectImage(ctx, "ghcr.io/invowk/invowk:v1.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "ghcr.io/invowk/invowk:v1.0.0")
	})

	t.Run("image not found error", func(t *testing.T) {
		recorder, cleanup := withMockExecCommandOutput(t, "", "Error: No such image", 1)
		defer cleanup()
		engine := newTestDockerEngine(t, recorder)

		_, err := engine.InspectImage(ctx, "nonexistent:latest")
		if err == nil {
			t.Fatal("expected error for nonexistent image")
		}
	})
}
