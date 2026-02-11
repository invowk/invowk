// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"errors"
	"testing"
)

func TestEngineNotAvailableError_Error(t *testing.T) {
	t.Parallel()

	err := &EngineNotAvailableError{
		Engine: "podman",
		Reason: "not installed",
	}

	expected := "container engine 'podman' is not available: not installed"
	if err.Error() != expected {
		t.Errorf("EngineNotAvailableError.Error() = %s, want %s", err.Error(), expected)
	}
}

func TestEngineNotAvailableError_UnwrapsToSentinel(t *testing.T) {
	t.Parallel()

	err := &EngineNotAvailableError{
		Engine: "docker",
		Reason: "not installed",
	}

	if !errors.Is(err, ErrNoEngineAvailable) {
		t.Error("EngineNotAvailableError should unwrap to ErrNoEngineAvailable")
	}
}

func TestErrNoEngineAvailable_Sentinel(t *testing.T) {
	t.Parallel()

	if ErrNoEngineAvailable == nil {
		t.Fatal("ErrNoEngineAvailable should not be nil")
	}
	if ErrNoEngineAvailable.Error() != "no container engine available" {
		t.Errorf("ErrNoEngineAvailable.Error() = %q, want %q", ErrNoEngineAvailable.Error(), "no container engine available")
	}
}

func TestDockerEngine_AvailableWithNoPath(t *testing.T) {
	t.Parallel()

	// Engine created with no binary path should not be available
	engine := &DockerEngine{BaseCLIEngine: NewBaseCLIEngine("")}
	if engine.Available() {
		t.Error("DockerEngine with empty path should not be available")
	}
}

func TestPodmanEngine_AvailableWithNoPath(t *testing.T) {
	t.Parallel()

	// Engine created with no binary path should not be available
	engine := &PodmanEngine{BaseCLIEngine: NewBaseCLIEngine("")}
	if engine.Available() {
		t.Error("PodmanEngine with empty path should not be available")
	}
}

func TestNewEngine_UnknownType(t *testing.T) {
	t.Parallel()

	_, err := NewEngine("unknown")
	if err == nil {
		t.Error("NewEngine with unknown type should return error")
	}
}

func TestNewEngine_Podman(t *testing.T) {
	t.Parallel()

	// This test verifies the logic, not actual availability
	engine, err := NewEngine(EngineTypePodman)
	// If neither podman nor docker is available, we should get an error
	if err != nil {
		if _, ok := errors.AsType[*EngineNotAvailableError](err); !ok {
			t.Errorf("expected EngineNotAvailableError, got %T", err)
		}
		return
	}

	// If we got an engine, it should be either podman or docker (fallback)
	if engine.Name() != "podman" && engine.Name() != "docker" {
		t.Errorf("expected podman or docker engine, got %s", engine.Name())
	}
}

func TestNewEngine_Docker(t *testing.T) {
	t.Parallel()

	// This test verifies the logic, not actual availability
	engine, err := NewEngine(EngineTypeDocker)
	// If neither docker nor podman is available, we should get an error
	if err != nil {
		if _, ok := errors.AsType[*EngineNotAvailableError](err); !ok {
			t.Errorf("expected EngineNotAvailableError, got %T", err)
		}
		return
	}

	// If we got an engine, it should be either docker or podman (fallback)
	if engine.Name() != "docker" && engine.Name() != "podman" {
		t.Errorf("expected docker or podman engine, got %s", engine.Name())
	}
}

func TestAutoDetectEngine(t *testing.T) {
	t.Parallel()

	engine, err := AutoDetectEngine()
	// If no engine is available, we should get an error
	if err != nil {
		if _, ok := errors.AsType[*EngineNotAvailableError](err); !ok {
			t.Errorf("expected EngineNotAvailableError, got %T: %v", err, err)
		}
		return
	}

	// If we got an engine, it should be either podman or docker
	if engine.Name() != "podman" && engine.Name() != "docker" {
		t.Errorf("expected podman or docker engine, got %s", engine.Name())
	}
}

// Integration tests - only run if container engine is available
func TestDockerEngine_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

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
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

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
