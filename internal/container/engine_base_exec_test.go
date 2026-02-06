// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// TestBaseCLIEngine_RunCommandStatus verifies RunCommandStatus returns only error status.
func TestBaseCLIEngine_RunCommandStatus(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		recorder := NewMockCommandRecorder()
		engine := NewBaseCLIEngine("/usr/bin/docker", WithExecCommand(recorder.ContextCommandFunc(t)))

		err := engine.RunCommandStatus(context.Background(), "image", "inspect", "myimage:latest")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		recorder.AssertInvocationCount(t, 1)
		recorder.AssertFirstArg(t, "image")
		recorder.AssertArgsContain(t, "inspect")
		recorder.AssertArgsContain(t, "myimage:latest")
	})

	t.Run("error wraps command failure", func(t *testing.T) {
		recorder := NewMockCommandRecorder()
		recorder.ExitCode = 1
		engine := NewBaseCLIEngine("/usr/bin/docker", WithExecCommand(recorder.ContextCommandFunc(t)))

		err := engine.RunCommandStatus(context.Background(), "rm", "-f", "container123")
		if err == nil {
			t.Fatal("expected error for non-zero exit code")
		}

		if !strings.Contains(err.Error(), "failed") {
			t.Errorf("error should indicate failure, got: %v", err)
		}
		if !strings.Contains(err.Error(), "docker") {
			t.Errorf("error should contain binary name, got: %v", err)
		}
	})
}

// TestBaseCLIEngine_RunCommandWithOutput verifies stdout capture via buffer.
func TestBaseCLIEngine_RunCommandWithOutput(t *testing.T) {
	t.Run("success captures stdout", func(t *testing.T) {
		recorder := NewMockCommandRecorder()
		recorder.Stdout = "27.0.1"
		engine := NewBaseCLIEngine("/usr/bin/docker", WithExecCommand(recorder.ContextCommandFunc(t)))

		out, err := engine.RunCommandWithOutput(context.Background(), "version", "--format", "{{.Server.Version}}")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !strings.Contains(out, "27.0.1") {
			t.Errorf("expected output to contain '27.0.1', got %q", out)
		}

		recorder.AssertInvocationCount(t, 1)
		recorder.AssertFirstArg(t, "version")
	})

	t.Run("error wraps command failure", func(t *testing.T) {
		recorder := NewMockCommandRecorder()
		recorder.ExitCode = 1
		engine := NewBaseCLIEngine("/usr/bin/docker", WithExecCommand(recorder.ContextCommandFunc(t)))

		out, err := engine.RunCommandWithOutput(context.Background(), "version")
		if err == nil {
			t.Fatal("expected error for non-zero exit code")
		}

		if out != "" {
			t.Errorf("expected empty output on error, got %q", out)
		}

		if !strings.Contains(err.Error(), "failed") {
			t.Errorf("error should indicate failure, got: %v", err)
		}
	})
}

// TestBaseCLIEngine_WithRunArgsTransformer verifies the run args transformer option.
func TestBaseCLIEngine_WithRunArgsTransformer(t *testing.T) {
	transformer := func(args []string) []string {
		// Simulate Podman's --userns=keep-id injection
		transformed := make([]string, 0, len(args)+1)
		for i, arg := range args {
			transformed = append(transformed, arg)
			// Insert --userns=keep-id right before the image name
			// (which comes after the last flag)
			if i == 0 && arg == "run" {
				transformed = append(transformed, "--userns=keep-id")
			}
		}
		return transformed
	}

	engine := NewBaseCLIEngine("/usr/bin/podman", WithRunArgsTransformer(transformer))

	args := engine.RunArgs(RunOptions{
		Image:   "debian:stable-slim",
		Command: []string{"echo", "hello"},
	})

	// Verify transformer was applied
	if !slices.Contains(args, "--userns=keep-id") {
		t.Errorf("expected --userns=keep-id in args, got: %v", args)
	}

	// Verify original args are still present
	if args[0] != "run" {
		t.Errorf("expected first arg 'run', got %q", args[0])
	}
}

// TestDockerEngine_Name verifies Docker engine reports correct name.
func TestDockerEngine_Name(t *testing.T) {
	engine := &DockerEngine{
		BaseCLIEngine: NewBaseCLIEngine(""),
	}

	if name := engine.Name(); name != "docker" {
		t.Errorf("DockerEngine.Name() = %q, want %q", name, "docker")
	}
}

// TestPodmanEngine_Name verifies Podman engine reports correct name.
func TestPodmanEngine_Name(t *testing.T) {
	engine := &PodmanEngine{
		BaseCLIEngine: NewBaseCLIEngine(""),
	}

	if name := engine.Name(); name != "podman" {
		t.Errorf("PodmanEngine.Name() = %q, want %q", name, "podman")
	}
}
