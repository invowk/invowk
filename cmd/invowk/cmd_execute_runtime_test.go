// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"io"
	"testing"

	"invowk-cli/internal/config"
	"invowk-cli/internal/discovery"
	"invowk-cli/pkg/invowkfile"
)

// newTestCommandService creates a commandService with no-op writers for testing.
func newTestCommandService() *commandService {
	return &commandService{
		stdout: io.Discard,
		stderr: io.Discard,
		ssh:    &sshServerController{},
	}
}

// buildDualRuntimeCommand creates a command with both native and virtual runtimes
// on all platforms. The first runtime listed (native) is the per-command default.
func buildDualRuntimeCommand() *discovery.CommandInfo {
	return &discovery.CommandInfo{
		Name: "test-cmd",
		Command: &invowkfile.Command{
			Name: "test-cmd",
			Implementations: []invowkfile.Implementation{
				{
					Script:    "echo hello",
					Platforms: invowkfile.AllPlatformConfigs(),
					Runtimes: []invowkfile.RuntimeConfig{
						{Name: invowkfile.RuntimeNative},
						{Name: invowkfile.RuntimeVirtual},
					},
				},
			},
		},
	}
}

// buildNativeOnlyCommand creates a command that only supports native runtime.
func buildNativeOnlyCommand() *discovery.CommandInfo {
	return &discovery.CommandInfo{
		Name: "native-only",
		Command: &invowkfile.Command{
			Name: "native-only",
			Implementations: []invowkfile.Implementation{
				{
					Script:    "echo native",
					Platforms: invowkfile.AllPlatformConfigs(),
					Runtimes: []invowkfile.RuntimeConfig{
						{Name: invowkfile.RuntimeNative},
					},
				},
			},
		},
	}
}

func TestResolveRuntime(t *testing.T) {
	t.Parallel()

	t.Run("config default runtime applied when command supports it", func(t *testing.T) {
		t.Parallel()

		svc := newTestCommandService()
		cmdInfo := buildDualRuntimeCommand()
		cfg := &config.Config{DefaultRuntime: "virtual"}

		resolved, err := svc.resolveRuntime(ExecuteRequest{Name: "test-cmd"}, cmdInfo, cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolved.mode != invowkfile.RuntimeVirtual {
			t.Errorf("expected runtime 'virtual', got %q", resolved.mode)
		}
	})

	t.Run("config default silently ignored when command does not support it", func(t *testing.T) {
		t.Parallel()

		svc := newTestCommandService()
		cmdInfo := buildNativeOnlyCommand()
		cfg := &config.Config{DefaultRuntime: "virtual"}

		resolved, err := svc.resolveRuntime(ExecuteRequest{Name: "native-only"}, cmdInfo, cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should fall through to per-command default (native).
		if resolved.mode != invowkfile.RuntimeNative {
			t.Errorf("expected runtime 'native' (per-command default), got %q", resolved.mode)
		}
	})

	t.Run("CLI flag overrides config default", func(t *testing.T) {
		t.Parallel()

		svc := newTestCommandService()
		cmdInfo := buildDualRuntimeCommand()
		cfg := &config.Config{DefaultRuntime: "virtual"}

		resolved, err := svc.resolveRuntime(
			ExecuteRequest{Name: "test-cmd", Runtime: "native"},
			cmdInfo,
			cfg,
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolved.mode != invowkfile.RuntimeNative {
			t.Errorf("expected runtime 'native' (CLI override), got %q", resolved.mode)
		}
	})

	t.Run("empty config default uses per-command default", func(t *testing.T) {
		t.Parallel()

		svc := newTestCommandService()
		cmdInfo := buildDualRuntimeCommand()
		cfg := &config.Config{DefaultRuntime: ""}

		resolved, err := svc.resolveRuntime(ExecuteRequest{Name: "test-cmd"}, cmdInfo, cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Per-command default is the first runtime listed: native.
		if resolved.mode != invowkfile.RuntimeNative {
			t.Errorf("expected runtime 'native' (per-command default), got %q", resolved.mode)
		}
	})

	t.Run("nil config uses per-command default", func(t *testing.T) {
		t.Parallel()

		svc := newTestCommandService()
		cmdInfo := buildDualRuntimeCommand()

		resolved, err := svc.resolveRuntime(ExecuteRequest{Name: "test-cmd"}, cmdInfo, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolved.mode != invowkfile.RuntimeNative {
			t.Errorf("expected runtime 'native' (per-command default), got %q", resolved.mode)
		}
	})
}
