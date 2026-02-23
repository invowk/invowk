// SPDX-License-Identifier: MPL-2.0

package container

import (
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

// TestDockerEngineBinaryPath tests the DockerEngine.BinaryPath method.
func TestDockerEngineBinaryPath(t *testing.T) {
	t.Parallel()
	engine := NewDockerEngine()
	if !engine.Available() {
		t.Skip("Docker not available")
	}

	path := engine.BinaryPath()
	if path == "" {
		t.Error("BinaryPath returned empty string for available engine")
	}
	if !strings.Contains(path, "docker") {
		t.Errorf("Expected path to contain 'docker', got: %s", path)
	}
}

// TestPodmanEngineBinaryPath tests the PodmanEngine.BinaryPath method.
func TestPodmanEngineBinaryPath(t *testing.T) {
	t.Parallel()
	engine := NewPodmanEngine()
	if !engine.Available() {
		t.Skip("Podman not available")
	}

	path := engine.BinaryPath()
	if path == "" {
		t.Error("BinaryPath returned empty string for available engine")
	}
	if !strings.Contains(path, "podman") {
		t.Errorf("Expected path to contain 'podman', got: %s", path)
	}
}

// TestDockerEngineBuildRunArgs tests the DockerEngine.BuildRunArgs method.
func TestDockerEngineBuildRunArgs(t *testing.T) {
	t.Parallel()
	engine := NewDockerEngine()

	opts := RunOptions{
		Image:       "debian:stable-slim",
		Command:     []string{"echo", "hello"},
		WorkDir:     "/workspace",
		Remove:      true,
		Interactive: true,
		TTY:         true,
		Env: map[string]string{
			"FOO": "bar",
		},
		Volumes:    []invowkfile.VolumeMountSpec{"/tmp:/tmp"},
		Ports:      []invowkfile.PortMappingSpec{"8080:80"},
		ExtraHosts: []HostMapping{"host.docker.internal:host-gateway"},
	}

	args := engine.BuildRunArgs(opts)

	// Verify 'run' is first
	if len(args) == 0 || args[0] != "run" {
		t.Errorf("Expected first arg to be 'run', got: %v", args)
	}

	// Check for expected flags
	argsStr := strings.Join(args, " ")

	expectedFlags := []string{
		"--rm",
		"-i",
		"-t",
		"-w /workspace",
		"-e FOO=bar",
		"-v /tmp:/tmp",
		"-p 8080:80",
		"--add-host host.docker.internal:host-gateway",
		"debian:stable-slim",
		"echo hello",
	}

	for _, flag := range expectedFlags {
		if !strings.Contains(argsStr, flag) {
			t.Errorf("Expected args to contain '%s', got: %s", flag, argsStr)
		}
	}
}

// TestPodmanEngineBuildRunArgs tests the PodmanEngine.BuildRunArgs method.
func TestPodmanEngineBuildRunArgs(t *testing.T) {
	t.Parallel()
	engine := NewPodmanEngine()

	opts := RunOptions{
		Image:       "debian:stable-slim",
		Command:     []string{"echo", "hello"},
		WorkDir:     "/workspace",
		Remove:      true,
		Interactive: true,
		TTY:         true,
		Env: map[string]string{
			"FOO": "bar",
		},
		Volumes:    []invowkfile.VolumeMountSpec{"/tmp:/tmp"},
		Ports:      []invowkfile.PortMappingSpec{"8080:80"},
		ExtraHosts: []HostMapping{"host.containers.internal:host-gateway"},
	}

	args := engine.BuildRunArgs(opts)

	// Verify 'run' is first
	if len(args) == 0 || args[0] != "run" {
		t.Errorf("Expected first arg to be 'run', got: %v", args)
	}

	// Check for expected flags
	argsStr := strings.Join(args, " ")

	expectedFlags := []string{
		"--rm",
		"-i",
		"-t",
		"-w /workspace",
		"-e FOO=bar",
		"-p 8080:80",
		"--add-host host.containers.internal:host-gateway",
		"debian:stable-slim",
		"echo hello",
	}

	for _, flag := range expectedFlags {
		if !strings.Contains(argsStr, flag) {
			t.Errorf("Expected args to contain '%s', got: %s", flag, argsStr)
		}
	}

	// Volume should have SELinux label (if SELinux is enabled)
	// Just verify the volume is present
	if !strings.Contains(argsStr, "/tmp:/tmp") {
		t.Errorf("Expected args to contain volume mount, got: %s", argsStr)
	}
}

// TestBuildRunArgsOptionalFlags tests that optional flags are correctly omitted.
func TestBuildRunArgsOptionalFlags(t *testing.T) {
	t.Parallel()
	engine := NewDockerEngine()

	// Minimal options
	opts := RunOptions{
		Image:   "debian:stable-slim",
		Command: []string{"sh"},
	}

	args := engine.BuildRunArgs(opts)
	argsStr := strings.Join(args, " ")

	// These should NOT be present
	unexpectedFlags := []string{
		"--rm",
		"-i",
		"-t",
		"-w",
		"-e",
		"-v",
		"-p",
		"--add-host",
		"--name",
	}

	for _, flag := range unexpectedFlags {
		if strings.Contains(argsStr, flag) {
			t.Errorf("Expected args to NOT contain '%s', got: %s", flag, argsStr)
		}
	}

	// Should contain image and command
	if !strings.Contains(argsStr, "debian:stable-slim") || !strings.Contains(argsStr, "sh") {
		t.Errorf("Expected args to contain image and command, got: %s", argsStr)
	}
}
