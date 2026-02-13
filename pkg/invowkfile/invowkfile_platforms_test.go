// SPDX-License-Identifier: MPL-2.0

package invowkfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePlatforms(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "build"
		implementations: [
			{
				script: "make build"
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
			}
		]
	},
	{
		name: "deploy"
		implementations: [
			{
				script: "deploy.sh"

				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 2 {
		t.Fatalf("Expected 2 commands, got %d", len(inv.Commands))
	}

	// First command - all platforms (explicitly listed)
	cmd1 := inv.Commands[0]
	platforms1 := cmd1.GetSupportedPlatforms()
	if len(platforms1) != 3 {
		t.Errorf("Expected 3 platforms for first command, got %d", len(platforms1))
	}

	// Second command - linux only
	cmd2 := inv.Commands[1]
	platforms2 := cmd2.GetSupportedPlatforms()
	if len(platforms2) != 1 {
		t.Errorf("Expected 1 platform for second command, got %d", len(platforms2))
	}
	if platforms2[0] != HostLinux {
		t.Errorf("First platform = %q, want %q", platforms2[0], HostLinux)
	}
}

func TestGenerateCUE_WithPlatforms(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{
			{
				Name: "build",
				Implementations: []Implementation{
					{Script: "make build", Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: []PlatformConfig{{Name: PlatformLinux}, {Name: PlatformMac}, {Name: PlatformWindows}}},
				},
			},
			{
				Name: "clean",
				Implementations: []Implementation{
					{Script: "rm -rf bin/", Runtimes: []RuntimeConfig{{Name: RuntimeNative}}, Platforms: []PlatformConfig{{Name: PlatformLinux}, {Name: PlatformMac}}},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	// Check that scripts structure is present
	if !strings.Contains(output, "implementations:") {
		t.Error("GenerateCUE should contain 'implementations:'")
	}

	if !strings.Contains(output, "runtimes:") {
		t.Error("GenerateCUE should contain 'runtimes:'")
	}

	if !strings.Contains(output, `"linux"`) {
		t.Error("GenerateCUE should contain 'linux'")
	}

	if !strings.Contains(output, `"macos"`) {
		t.Error("GenerateCUE should contain 'macos'")
	}
}

// Tests for enable_host_ssh functionality

func TestParseEnableHostSSH(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "container-ssh"
		description: "Container command with host SSH enabled"
		implementations: [
			{
				script: "echo hello"

				runtimes: [{name: "container", image: "debian:stable-slim", enable_host_ssh: true}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(inv.Commands) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(inv.Commands))
	}

	cmd := inv.Commands[0]
	if len(cmd.Implementations) != 1 {
		t.Fatalf("Expected 1 implementation, got %d", len(cmd.Implementations))
	}

	impl := cmd.Implementations[0]
	if len(impl.Runtimes) != 1 {
		t.Fatalf("Expected 1 runtime, got %d", len(impl.Runtimes))
	}

	rt := impl.Runtimes[0]
	if rt.Name != RuntimeContainer {
		t.Errorf("Runtime name = %q, want %q", rt.Name, RuntimeContainer)
	}

	if !rt.EnableHostSSH {
		t.Error("EnableHostSSH should be true")
	}

	if rt.Image != "debian:stable-slim" {
		t.Errorf("Image = %q, want %q", rt.Image, "debian:stable-slim")
	}
}

func TestParseEnableHostSSH_DefaultFalse(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "container-no-ssh"
		implementations: [
			{
				script: "echo hello"

				runtimes: [{name: "container", image: "debian:stable-slim"}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	cmd := inv.Commands[0]
	rt := cmd.Implementations[0].Runtimes[0]

	if rt.EnableHostSSH {
		t.Error("EnableHostSSH should be false by default")
	}
}

func TestScript_HasHostSSH(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		script   Implementation
		expected bool
	}{
		{
			name: "container with enable_host_ssh true",
			script: Implementation{
				Script: "echo test",

				Runtimes:  []RuntimeConfig{{Name: RuntimeContainer, EnableHostSSH: true, Image: "debian:stable-slim"}},
				Platforms: []PlatformConfig{{Name: PlatformLinux}},
			},
			expected: true,
		},
		{
			name: "container with enable_host_ssh false",
			script: Implementation{
				Script: "echo test",

				Runtimes:  []RuntimeConfig{{Name: RuntimeContainer, EnableHostSSH: false, Image: "debian:stable-slim"}},
				Platforms: []PlatformConfig{{Name: PlatformLinux}},
			},
			expected: false,
		},
		{
			name: "native runtime (no enable_host_ssh)",
			script: Implementation{
				Script: "echo test",

				Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
				Platforms: []PlatformConfig{{Name: PlatformLinux}},
			},
			expected: false,
		},
		{
			name: "multiple runtimes, one with enable_host_ssh",
			script: Implementation{
				Script: "echo test",

				Runtimes: []RuntimeConfig{
					{Name: RuntimeNative},
					{Name: RuntimeContainer, EnableHostSSH: true, Image: "debian:stable-slim"},
				},
				Platforms: []PlatformConfig{{Name: PlatformLinux}},
			},
			expected: true,
		},
		{
			name: "multiple container runtimes, none with enable_host_ssh",
			script: Implementation{
				Script: "echo test",

				Runtimes: []RuntimeConfig{
					{Name: RuntimeContainer, EnableHostSSH: false, Image: "debian:stable-slim"},
					{Name: RuntimeNative},
				},
				Platforms: []PlatformConfig{{Name: PlatformLinux}},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.script.HasHostSSH()
			if result != tt.expected {
				t.Errorf("HasHostSSH() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestScript_GetHostSSHForRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		script   Implementation
		runtime  RuntimeMode
		expected bool
	}{
		{
			name: "container runtime with enable_host_ssh true",
			script: Implementation{
				Script: "echo test",

				Runtimes:  []RuntimeConfig{{Name: RuntimeContainer, EnableHostSSH: true, Image: "debian:stable-slim"}},
				Platforms: []PlatformConfig{{Name: PlatformLinux}},
			},
			runtime:  RuntimeContainer,
			expected: true,
		},
		{
			name: "container runtime with enable_host_ssh false",
			script: Implementation{
				Script: "echo test",

				Runtimes:  []RuntimeConfig{{Name: RuntimeContainer, EnableHostSSH: false, Image: "debian:stable-slim"}},
				Platforms: []PlatformConfig{{Name: PlatformLinux}},
			},
			runtime:  RuntimeContainer,
			expected: false,
		},
		{
			name: "native runtime always returns false",
			script: Implementation{
				Script: "echo test",

				Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
				Platforms: []PlatformConfig{{Name: PlatformLinux}},
			},
			runtime:  RuntimeNative,
			expected: false,
		},
		{
			name: "virtual runtime always returns false",
			script: Implementation{
				Script: "echo test",

				Runtimes:  []RuntimeConfig{{Name: RuntimeVirtual}},
				Platforms: []PlatformConfig{{Name: PlatformLinux}},
			},
			runtime:  RuntimeVirtual,
			expected: false,
		},
		{
			name: "runtime not found returns false",
			script: Implementation{
				Script: "echo test",

				Runtimes:  []RuntimeConfig{{Name: RuntimeNative}},
				Platforms: []PlatformConfig{{Name: PlatformLinux}},
			},
			runtime:  RuntimeContainer,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := tt.script.GetHostSSHForRuntime(tt.runtime)
			if result != tt.expected {
				t.Errorf("GetHostSSHForRuntime(%s) = %v, want %v", tt.runtime, result, tt.expected)
			}
		})
	}
}

func TestValidateEnableHostSSH_InvalidForNonContainer(t *testing.T) {
	t.Parallel()

	// Test that enable_host_ssh is rejected for non-container runtimes
	// This tests the Go validation, not the CUE schema (CUE schema only allows enable_host_ssh for container)

	rt := &RuntimeConfig{
		Name:          RuntimeNative,
		EnableHostSSH: true, // Invalid for native runtime
	}

	err := validateRuntimeConfig(rt, "test-cmd", 1)
	if err == nil {
		t.Error("Expected error for enable_host_ssh on native runtime, got nil")
	}

	if !strings.Contains(err.Error(), "enable_host_ssh") {
		t.Errorf("Error should mention enable_host_ssh, got: %v", err)
	}
}

func TestValidateEnableHostSSH_ValidForContainer(t *testing.T) {
	t.Parallel()

	rt := &RuntimeConfig{
		Name:          RuntimeContainer,
		EnableHostSSH: true,
		Image:         "debian:stable-slim",
	}

	err := validateRuntimeConfig(rt, "test-cmd", 1)
	if err != nil {
		t.Errorf("Unexpected error for enable_host_ssh on container runtime: %v", err)
	}
}

func TestGenerateCUE_WithEnableHostSSH(t *testing.T) {
	t.Parallel()

	inv := &Invowkfile{
		Commands: []Command{
			{
				Name: "container-ssh",
				Implementations: []Implementation{
					{
						Script: "echo hello",

						Runtimes:  []RuntimeConfig{{Name: RuntimeContainer, EnableHostSSH: true, Image: "debian:stable-slim"}},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
					},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	if !strings.Contains(output, "enable_host_ssh: true") {
		t.Error("GenerateCUE should contain 'enable_host_ssh: true'")
	}

	if !strings.Contains(output, `image: "debian:stable-slim"`) {
		t.Error("GenerateCUE should contain image specification")
	}
}

func TestGenerateCUE_WithEnableHostSSH_False(t *testing.T) {
	t.Parallel()

	// When enable_host_ssh is false (default), it should not appear in the output
	inv := &Invowkfile{
		Commands: []Command{
			{
				Name: "container-no-ssh",
				Implementations: []Implementation{
					{
						Script: "echo hello",

						Runtimes:  []RuntimeConfig{{Name: RuntimeContainer, EnableHostSSH: false, Image: "debian:stable-slim"}},
						Platforms: []PlatformConfig{{Name: PlatformLinux}},
					},
				},
			},
		},
	}

	output := GenerateCUE(inv)

	if strings.Contains(output, "enable_host_ssh") {
		t.Error("GenerateCUE should not contain 'enable_host_ssh' when it's false")
	}
}

func TestParseContainerRuntimeWithAllOptions(t *testing.T) {
	t.Parallel()

	cueContent := `
cmds: [
	{
		name: "full-container"
		implementations: [
			{
				script: "echo hello"

				runtimes: [{
					name: "container"
					image: "golang:1.21"
					enable_host_ssh: true
					volumes: ["./data:/data", "/tmp:/tmp:ro"]
					ports: ["8080:80", "3000:3000"]
				}]
				platforms: [{name: "linux"}]
			}
		]
	}
]
`

	tmpDir, err := os.MkdirTemp("", "invowk-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }() // Cleanup temp dir; error non-critical

	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	inv, err := Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	cmd := inv.Commands[0]
	rt := cmd.Implementations[0].Runtimes[0]

	if rt.Name != RuntimeContainer {
		t.Errorf("Runtime name = %q, want %q", rt.Name, RuntimeContainer)
	}

	if rt.Image != "golang:1.21" {
		t.Errorf("Image = %q, want %q", rt.Image, "golang:1.21")
	}

	if !rt.EnableHostSSH {
		t.Error("EnableHostSSH should be true")
	}

	if len(rt.Volumes) != 2 {
		t.Errorf("Volumes length = %d, want 2", len(rt.Volumes))
	} else {
		if rt.Volumes[0] != "./data:/data" {
			t.Errorf("Volumes[0] = %q, want %q", rt.Volumes[0], "./data:/data")
		}
		if rt.Volumes[1] != "/tmp:/tmp:ro" {
			t.Errorf("Volumes[1] = %q, want %q", rt.Volumes[1], "/tmp:/tmp:ro")
		}
	}

	if len(rt.Ports) != 2 {
		t.Errorf("Ports length = %d, want 2", len(rt.Ports))
	} else {
		if rt.Ports[0] != "8080:80" {
			t.Errorf("Ports[0] = %q, want %q", rt.Ports[0], "8080:80")
		}
		if rt.Ports[1] != "3000:3000" {
			t.Errorf("Ports[1] = %q, want %q", rt.Ports[1], "3000:3000")
		}
	}
}

func TestMissingPlatformsRejected(t *testing.T) {
	t.Parallel()

	// CUE schema now requires at least one platform per implementation.
	// An implementation without platforms should fail CUE validation.
	cueContent := `
cmds: [
	{
		name: "no-platforms"
		implementations: [
			{
				script: "echo test"
				runtimes: [{name: "native"}]
			}
		]
	}
]
`
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if writeErr := os.WriteFile(invowkfilePath, []byte(cueContent), 0o644); writeErr != nil {
		t.Fatalf("Failed to write invowkfile: %v", writeErr)
	}

	_, err := Parse(invowkfilePath)
	if err == nil {
		t.Error("Parse() should reject implementation without platforms")
	}
}
