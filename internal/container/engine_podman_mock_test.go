// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
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
	recorder, cleanup := withMockExecCommand(t)
	defer cleanup()

	engine := newTestPodmanEngine(t, recorder)
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
		recorder.AssertCommandName(t, "/usr/bin/podman")
		recorder.AssertFirstArg(t, "build")
		recorder.AssertArgsContain(t, "-t")
		recorder.AssertArgsContain(t, "myimage:latest")
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
}

// TestPodmanEngine_Run_Arguments verifies Podman Run() constructs correct arguments.
func TestPodmanEngine_Run_Arguments(t *testing.T) {
	recorder, cleanup := withMockExecCommand(t)
	defer cleanup()

	engine := newTestPodmanEngine(t, recorder)
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
		recorder.AssertCommandName(t, "/usr/bin/podman")
		recorder.AssertFirstArg(t, "run")
		recorder.AssertArgsContain(t, "debian:stable-slim")
	})

	t.Run("with all options", func(t *testing.T) {
		recorder.Reset()
		opts := RunOptions{
			Image:       "debian:stable-slim",
			Command:     []string{"bash", "-c", "echo test"},
			WorkDir:     "/app",
			Name:        "podman-test",
			Remove:      true,
			Interactive: true,
			TTY:         true,
			Env:         map[string]string{"VAR": "value"},
			Volumes:     []string{"/src:/src"},
			Ports:       []string{"8080:80"},
			ExtraHosts:  []string{"host.containers.internal:host-gateway"},
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
	recorder, cleanup := withMockExecCommand(t)
	defer cleanup()

	engine := newTestPodmanEngine(t, recorder)
	ctx := context.Background()

	t.Run("podman uses image exists command", func(t *testing.T) {
		recorder.Reset()

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
	ctx := context.Background()

	t.Run("build failure", func(t *testing.T) {
		recorder, cleanup := withMockExecCommandOutput(t, "", "Error: build failed", 1)
		defer cleanup()
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
		recorder, cleanup := withMockExecCommandOutput(t, "", "Error: No such image", 1)
		defer cleanup()
		engine := newTestPodmanEngine(t, recorder)

		exists, err := engine.ImageExists(ctx, "nonexistent:latest")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if exists {
			t.Error("expected image to not exist")
		}
	})
}

// TestPodmanEngine_Version_Arguments verifies Podman Version() uses different format.
func TestPodmanEngine_Version_Arguments(t *testing.T) {
	recorder, cleanup := withMockExecCommandOutput(t, "5.0.0", "", 0)
	defer cleanup()

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
	recorder, cleanup := withMockExecCommand(t)
	defer cleanup()

	engine := newTestPodmanEngine(t, recorder)
	ctx := context.Background()

	t.Run("basic remove", func(t *testing.T) {
		recorder.Reset()

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
		recorder.Reset()

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
	recorder, cleanup := withMockExecCommand(t)
	defer cleanup()

	engine := newTestPodmanEngine(t, recorder)
	ctx := context.Background()

	t.Run("basic remove image", func(t *testing.T) {
		recorder.Reset()

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
		recorder.Reset()

		err := engine.RemoveImage(ctx, "myimage:v2", true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertArgsContain(t, "-f")
		recorder.AssertArgsContain(t, "myimage:v2")
	})
}
