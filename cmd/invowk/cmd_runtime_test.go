// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"strings"
	"testing"

	"github.com/invowk/invowk/pkg/invowkfile"
)

// ---------------------------------------------------------------------------
// Platform and runtime tests
// ---------------------------------------------------------------------------

func TestCommand_CanRunOnCurrentHost(t *testing.T) {
	currentOS := invowkfile.CurrentPlatform()

	tests := []struct {
		name     string
		cmd      *invowkfile.Command
		expected bool
	}{
		{
			name: "current host supported",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}, Platforms: []invowkfile.PlatformConfig{{Name: currentOS}}},
				},
			},
			expected: true,
		},
		{
			name: "current host not supported",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}, Platforms: []invowkfile.PlatformConfig{{Name: "nonexistent"}}},
				},
			},
			expected: false,
		},
		{
			name: "all hosts supported (all platforms specified)",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}, Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}, {Name: invowkfile.PlatformWindows}}},
				},
			},
			expected: true,
		},
		{
			name: "empty scripts list",
			cmd: &invowkfile.Command{
				Name:            "test",
				Implementations: []invowkfile.Implementation{},
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
		cmd      *invowkfile.Command
		expected string
	}{
		{
			name: "single platform",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}, Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}}},
				},
			},
			expected: "linux",
		},
		{
			name: "multiple platforms",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}, Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}}},
				},
			},
			expected: "linux, macos",
		},
		{
			name: "all platforms (all platforms specified)",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}, Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}, {Name: invowkfile.PlatformWindows}}},
				},
			},
			expected: "linux, macos, windows",
		},
		{
			name: "empty scripts",
			cmd: &invowkfile.Command{
				Name:            "test",
				Implementations: []invowkfile.Implementation{},
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

func TestCurrentPlatform(t *testing.T) {
	// Just verify it returns one of the expected values
	currentOS := invowkfile.CurrentPlatform()
	validOSes := map[invowkfile.PlatformType]bool{
		invowkfile.PlatformLinux:   true,
		invowkfile.PlatformMac:     true,
		invowkfile.PlatformWindows: true,
	}

	if !validOSes[currentOS] {
		t.Errorf("CurrentPlatform() returned unexpected value: %q", currentOS)
	}
}

func TestCommand_GetDefaultRuntimeForPlatform(t *testing.T) {
	currentPlatform := invowkfile.CurrentPlatform()

	tests := []struct {
		name     string
		cmd      *invowkfile.Command
		expected invowkfile.RuntimeMode
	}{
		{
			name: "first runtime is default",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}, {Name: invowkfile.RuntimeContainer}}, Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}, {Name: invowkfile.PlatformWindows}}},
				},
			},
			expected: invowkfile.RuntimeNative,
		},
		{
			name: "container as default",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer}, {Name: invowkfile.RuntimeNative}}, Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}, {Name: invowkfile.PlatformWindows}}},
				},
			},
			expected: invowkfile.RuntimeContainer,
		},
		{
			name: "empty scripts returns native",
			cmd: &invowkfile.Command{
				Name:            "test",
				Implementations: []invowkfile.Implementation{},
			},
			expected: invowkfile.RuntimeNative,
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
	currentPlatform := invowkfile.CurrentPlatform()

	cmd := &invowkfile.Command{
		Name: "test",
		Implementations: []invowkfile.Implementation{
			{Script: "echo", Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}, {Name: invowkfile.RuntimeVirtual}}, Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}, {Name: invowkfile.PlatformWindows}}},
		},
	}

	tests := []struct {
		runtime  invowkfile.RuntimeMode
		expected bool
	}{
		{invowkfile.RuntimeNative, true},
		{invowkfile.RuntimeVirtual, true},
		{invowkfile.RuntimeContainer, false},
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
	currentPlatform := invowkfile.CurrentPlatform()

	tests := []struct {
		name     string
		cmd      *invowkfile.Command
		expected string
	}{
		{
			name: "single runtime with asterisk",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}}, Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}, {Name: invowkfile.PlatformWindows}}},
				},
			},
			expected: "native*",
		},
		{
			name: "multiple runtimes with first marked",
			cmd: &invowkfile.Command{
				Name: "test",
				Implementations: []invowkfile.Implementation{
					{Script: "echo", Runtimes: []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeNative}, {Name: invowkfile.RuntimeContainer}}, Platforms: []invowkfile.PlatformConfig{{Name: invowkfile.PlatformLinux}, {Name: invowkfile.PlatformMac}, {Name: invowkfile.PlatformWindows}}},
				},
			},
			expected: "native*, container",
		},
		{
			name: "empty scripts",
			cmd: &invowkfile.Command{
				Name:            "test",
				Implementations: []invowkfile.Implementation{},
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
