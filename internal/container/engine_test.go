// SPDX-License-Identifier: EPL-2.0

// Package container provides an abstraction layer for container runtimes (Docker/Podman).
package container

import (
	"context"
	"testing"
)

func TestEngineType_Constants(t *testing.T) {
	if EngineTypePodman != "podman" {
		t.Errorf("EngineTypePodman = %s, want podman", EngineTypePodman)
	}

	if EngineTypeDocker != "docker" {
		t.Errorf("EngineTypeDocker = %s, want docker", EngineTypeDocker)
	}
}

func TestErrEngineNotAvailable_Error(t *testing.T) {
	err := &ErrEngineNotAvailable{
		Engine: "podman",
		Reason: "not installed",
	}

	expected := "container engine 'podman' is not available: not installed"
	if err.Error() != expected {
		t.Errorf("ErrEngineNotAvailable.Error() = %s, want %s", err.Error(), expected)
	}
}

func TestDockerEngine_Name(t *testing.T) {
	engine := NewDockerEngine()
	if engine.Name() != "docker" {
		t.Errorf("DockerEngine.Name() = %s, want docker", engine.Name())
	}
}

func TestPodmanEngine_Name(t *testing.T) {
	engine := NewPodmanEngine()
	if engine.Name() != "podman" {
		t.Errorf("PodmanEngine.Name() = %s, want podman", engine.Name())
	}
}

func TestDockerEngine_AvailableWithNoPath(t *testing.T) {
	// Engine created with no binary path should not be available
	engine := &DockerEngine{binaryPath: ""}
	if engine.Available() {
		t.Error("DockerEngine with empty path should not be available")
	}
}

func TestPodmanEngine_AvailableWithNoPath(t *testing.T) {
	// Engine created with no binary path should not be available
	engine := &PodmanEngine{binaryPath: ""}
	if engine.Available() {
		t.Error("PodmanEngine with empty path should not be available")
	}
}

func TestNewEngine_UnknownType(t *testing.T) {
	_, err := NewEngine("unknown")
	if err == nil {
		t.Error("NewEngine with unknown type should return error")
	}
}

func TestNewEngine_Podman(t *testing.T) {
	// This test verifies the logic, not actual availability
	engine, err := NewEngine(EngineTypePodman)

	// If neither podman nor docker is available, we should get an error
	if err != nil {
		if _, ok := err.(*ErrEngineNotAvailable); !ok {
			t.Errorf("expected ErrEngineNotAvailable, got %T", err)
		}
		return
	}

	// If we got an engine, it should be either podman or docker (fallback)
	if engine.Name() != "podman" && engine.Name() != "docker" {
		t.Errorf("expected podman or docker engine, got %s", engine.Name())
	}
}

func TestNewEngine_Docker(t *testing.T) {
	// This test verifies the logic, not actual availability
	engine, err := NewEngine(EngineTypeDocker)

	// If neither docker nor podman is available, we should get an error
	if err != nil {
		if _, ok := err.(*ErrEngineNotAvailable); !ok {
			t.Errorf("expected ErrEngineNotAvailable, got %T", err)
		}
		return
	}

	// If we got an engine, it should be either docker or podman (fallback)
	if engine.Name() != "docker" && engine.Name() != "podman" {
		t.Errorf("expected docker or podman engine, got %s", engine.Name())
	}
}

func TestAutoDetectEngine(t *testing.T) {
	engine, err := AutoDetectEngine()

	// If no engine is available, we should get an error
	if err != nil {
		if _, ok := err.(*ErrEngineNotAvailable); !ok {
			t.Errorf("expected ErrEngineNotAvailable, got %T: %v", err, err)
		}
		return
	}

	// If we got an engine, it should be either podman or docker
	if engine.Name() != "podman" && engine.Name() != "docker" {
		t.Errorf("expected podman or docker engine, got %s", engine.Name())
	}
}

func TestBuildOptions_Defaults(t *testing.T) {
	opts := BuildOptions{
		ContextDir: "/tmp/build",
		Tag:        "myimage:latest",
	}

	if opts.ContextDir != "/tmp/build" {
		t.Errorf("ContextDir = %s, want /tmp/build", opts.ContextDir)
	}

	if opts.Tag != "myimage:latest" {
		t.Errorf("Tag = %s, want myimage:latest", opts.Tag)
	}

	if opts.NoCache {
		t.Error("NoCache should default to false")
	}

	if opts.Dockerfile != "" {
		t.Errorf("Dockerfile should default to empty, got %s", opts.Dockerfile)
	}
}

func TestRunOptions_Defaults(t *testing.T) {
	opts := RunOptions{
		Image:   "alpine:latest",
		Command: []string{"echo", "hello"},
	}

	if opts.Image != "alpine:latest" {
		t.Errorf("Image = %s, want alpine:latest", opts.Image)
	}

	if len(opts.Command) != 2 {
		t.Errorf("Command length = %d, want 2", len(opts.Command))
	}

	if opts.Remove {
		t.Error("Remove should default to false")
	}

	if opts.Interactive {
		t.Error("Interactive should default to false")
	}

	if opts.TTY {
		t.Error("TTY should default to false")
	}
}

func TestRunResult_Defaults(t *testing.T) {
	result := &RunResult{}

	if result.ContainerID != "" {
		t.Errorf("ContainerID should default to empty, got %s", result.ContainerID)
	}

	if result.ExitCode != 0 {
		t.Errorf("ExitCode should default to 0, got %d", result.ExitCode)
	}

	if result.Error != nil {
		t.Errorf("Error should default to nil, got %v", result.Error)
	}
}

// Integration tests - only run if container engine is available
func TestDockerEngine_Integration(t *testing.T) {
	engine := NewDockerEngine()
	if !engine.Available() {
		t.Skip("Docker is not available, skipping integration tests")
	}

	ctx := context.Background()

	t.Run("Version", func(t *testing.T) {
		version, err := engine.Version(ctx)
		if err != nil {
			t.Errorf("Version() returned error: %v", err)
		}
		if version == "" {
			t.Error("Version() returned empty string")
		}
		t.Logf("Docker version: %s", version)
	})

	t.Run("ImageExists_NonExistent", func(t *testing.T) {
		exists, err := engine.ImageExists(ctx, "invowk-test-nonexistent-image:latest")
		if err != nil {
			t.Errorf("ImageExists() returned error: %v", err)
		}
		if exists {
			t.Error("ImageExists() returned true for non-existent image")
		}
	})
}

func TestPodmanEngine_Integration(t *testing.T) {
	engine := NewPodmanEngine()
	if !engine.Available() {
		t.Skip("Podman is not available, skipping integration tests")
	}

	ctx := context.Background()

	t.Run("Version", func(t *testing.T) {
		version, err := engine.Version(ctx)
		if err != nil {
			t.Errorf("Version() returned error: %v", err)
		}
		if version == "" {
			t.Error("Version() returned empty string")
		}
		t.Logf("Podman version: %s", version)
	})

	t.Run("ImageExists_NonExistent", func(t *testing.T) {
		exists, err := engine.ImageExists(ctx, "invowk-test-nonexistent-image:latest")
		if err != nil {
			t.Errorf("ImageExists() returned error: %v", err)
		}
		if exists {
			t.Error("ImageExists() returned true for non-existent image")
		}
	})
}
