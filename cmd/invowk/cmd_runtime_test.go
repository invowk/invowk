// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"strings"
	"testing"

	"invowk-cli/pkg/invkfile"
)

// ---------------------------------------------------------------------------
// Platform and runtime tests
// ---------------------------------------------------------------------------

func TestCommand_CanRunOnCurrentHost(t *testing.T) {
	currentOS := invkfile.GetCurrentHostOS()

	tests := []struct {
		name     string
		cmd      *invkfile.Command
		expected bool
	}{
		{
			name: "current host supported",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}, Platforms: []invkfile.PlatformConfig{{Name: currentOS}}},
				},
			},
			expected: true,
		},
		{
			name: "current host not supported",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}, Platforms: []invkfile.PlatformConfig{{Name: "nonexistent"}}},
				},
			},
			expected: false,
		},
		{
			name: "all hosts supported (all platforms specified)",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}, Platforms: []invkfile.PlatformConfig{{Name: invkfile.PlatformLinux}, {Name: invkfile.PlatformMac}, {Name: invkfile.PlatformWindows}}},
				},
			},
			expected: true,
		},
		{
			name: "empty scripts list",
			cmd: &invkfile.Command{
				Name:            "test",
				Implementations: []invkfile.Implementation{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cmd.CanRunOnCurrentHost()
			if result != tt.expected {
				t.Errorf("CanRunOnCurrentHost() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCommand_GetPlatformsString(t *testing.T) {
	tests := []struct {
		name     string
		cmd      *invkfile.Command
		expected string
	}{
		{
			name: "single platform",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}, Platforms: []invkfile.PlatformConfig{{Name: invkfile.PlatformLinux}}},
				},
			},
			expected: "linux",
		},
		{
			name: "multiple platforms",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}, Platforms: []invkfile.PlatformConfig{{Name: invkfile.PlatformLinux}, {Name: invkfile.PlatformMac}}},
				},
			},
			expected: "linux, macos",
		},
		{
			name: "all platforms (all platforms specified)",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}, Platforms: []invkfile.PlatformConfig{{Name: invkfile.PlatformLinux}, {Name: invkfile.PlatformMac}, {Name: invkfile.PlatformWindows}}},
				},
			},
			expected: "linux, macos, windows",
		},
		{
			name: "empty scripts",
			cmd: &invkfile.Command{
				Name:            "test",
				Implementations: []invkfile.Implementation{},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cmd.GetPlatformsString()
			if result != tt.expected {
				t.Errorf("GetPlatformsString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetCurrentHostOS(t *testing.T) {
	// Just verify it returns one of the expected values
	currentOS := invkfile.GetCurrentHostOS()
	validOSes := map[invkfile.HostOS]bool{
		invkfile.HostLinux:   true,
		invkfile.HostMac:     true,
		invkfile.HostWindows: true,
	}

	if !validOSes[currentOS] {
		t.Errorf("GetCurrentHostOS() returned unexpected value: %q", currentOS)
	}
}

func TestCommand_GetDefaultRuntimeForPlatform(t *testing.T) {
	currentPlatform := invkfile.GetCurrentHostOS()

	tests := []struct {
		name     string
		cmd      *invkfile.Command
		expected invkfile.RuntimeMode
	}{
		{
			name: "first runtime is default",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}, {Name: invkfile.RuntimeContainer}}, Platforms: []invkfile.PlatformConfig{{Name: invkfile.PlatformLinux}, {Name: invkfile.PlatformMac}, {Name: invkfile.PlatformWindows}}},
				},
			},
			expected: invkfile.RuntimeNative,
		},
		{
			name: "container as default",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeContainer}, {Name: invkfile.RuntimeNative}}, Platforms: []invkfile.PlatformConfig{{Name: invkfile.PlatformLinux}, {Name: invkfile.PlatformMac}, {Name: invkfile.PlatformWindows}}},
				},
			},
			expected: invkfile.RuntimeContainer,
		},
		{
			name: "empty scripts returns native",
			cmd: &invkfile.Command{
				Name:            "test",
				Implementations: []invkfile.Implementation{},
			},
			expected: invkfile.RuntimeNative,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cmd.GetDefaultRuntimeForPlatform(currentPlatform)
			if result != tt.expected {
				t.Errorf("GetDefaultRuntimeForPlatform() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCommand_IsRuntimeAllowedForPlatform(t *testing.T) {
	currentPlatform := invkfile.GetCurrentHostOS()

	cmd := &invkfile.Command{
		Name: "test",
		Implementations: []invkfile.Implementation{
			{Script: "echo", Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}, {Name: invkfile.RuntimeVirtual}}, Platforms: []invkfile.PlatformConfig{{Name: invkfile.PlatformLinux}, {Name: invkfile.PlatformMac}, {Name: invkfile.PlatformWindows}}},
		},
	}

	tests := []struct {
		runtime  invkfile.RuntimeMode
		expected bool
	}{
		{invkfile.RuntimeNative, true},
		{invkfile.RuntimeVirtual, true},
		{invkfile.RuntimeContainer, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.runtime), func(t *testing.T) {
			result := cmd.IsRuntimeAllowedForPlatform(currentPlatform, tt.runtime)
			if result != tt.expected {
				t.Errorf("IsRuntimeAllowedForPlatform(%v) = %v, want %v", tt.runtime, result, tt.expected)
			}
		})
	}
}

func TestCommand_GetRuntimesStringForPlatform(t *testing.T) {
	currentPlatform := invkfile.GetCurrentHostOS()

	tests := []struct {
		name     string
		cmd      *invkfile.Command
		expected string
	}{
		{
			name: "single runtime with asterisk",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}}, Platforms: []invkfile.PlatformConfig{{Name: invkfile.PlatformLinux}, {Name: invkfile.PlatformMac}, {Name: invkfile.PlatformWindows}}},
				},
			},
			expected: "native*",
		},
		{
			name: "multiple runtimes with first marked",
			cmd: &invkfile.Command{
				Name: "test",
				Implementations: []invkfile.Implementation{
					{Script: "echo", Runtimes: []invkfile.RuntimeConfig{{Name: invkfile.RuntimeNative}, {Name: invkfile.RuntimeContainer}}, Platforms: []invkfile.PlatformConfig{{Name: invkfile.PlatformLinux}, {Name: invkfile.PlatformMac}, {Name: invkfile.PlatformWindows}}},
				},
			},
			expected: "native*, container",
		},
		{
			name: "empty scripts",
			cmd: &invkfile.Command{
				Name:            "test",
				Implementations: []invkfile.Implementation{},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.cmd.GetRuntimesStringForPlatform(currentPlatform)
			if result != tt.expected {
				t.Errorf("GetRuntimesStringForPlatform() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestRenderRuntimeNotAllowedError(t *testing.T) {
	output := RenderRuntimeNotAllowedError("build", "container", "native, virtual")

	if !strings.Contains(output, "Runtime not allowed") {
		t.Error("RenderRuntimeNotAllowedError should contain 'Runtime not allowed'")
	}

	if !strings.Contains(output, "'build'") {
		t.Error("RenderRuntimeNotAllowedError should contain command name")
	}

	if !strings.Contains(output, "container") {
		t.Error("RenderRuntimeNotAllowedError should contain selected runtime")
	}

	if !strings.Contains(output, "native, virtual") {
		t.Error("RenderRuntimeNotAllowedError should contain allowed runtimes")
	}
}
