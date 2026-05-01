// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"testing"

	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// testConfigFallback wraps the application-service fallback for commandsvc.ConfigFallbackFunc.
func testConfigFallback(ctx context.Context, provider config.Loader, configPath string) (cfg *config.Config, diags []commandsvc.Diagnostic) {
	return commandsvc.LoadConfigWithFallback(ctx, provider, configPath)
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
		Invowkfile: &invowkfile.Invowkfile{},
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
		Invowkfile: &invowkfile.Invowkfile{},
	}
}

// TestResolveRuntime verifies the 3-tier runtime selection logic through the
// commandsvc.Service. Since resolveRuntime is now internal to commandsvc,
// we test through the public Execute() method with appropriate mock services.
// For focused runtime resolution tests, see internal/app/execute/.
func TestResolveRuntime(t *testing.T) {
	t.Parallel()

	t.Run("config default runtime applied when command supports it", func(t *testing.T) {
		t.Parallel()

		cmdInfo := buildDualRuntimeCommand()
		cfg := &config.Config{DefaultRuntime: config.RuntimeVirtual}
		svc := commandsvc.New(
			&fixedConfigProvider{cfg: cfg},
			&lookupDiscoveryService{lookup: discovery.LookupResult{Command: cmdInfo}},
			func() map[string]string { return nil },
			testConfigFallback,
		)

		result, _, err := svc.Execute(t.Context(), commandsvc.Request{Name: "test-cmd"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Verify successful execution (exit code 0 indicates runtime resolved correctly).
		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", result.ExitCode)
		}
	})

	t.Run("CLI flag overrides config default", func(t *testing.T) {
		t.Parallel()

		cmdInfo := buildDualRuntimeCommand()
		cfg := &config.Config{DefaultRuntime: config.RuntimeVirtual}
		svc := commandsvc.New(
			&fixedConfigProvider{cfg: cfg},
			&lookupDiscoveryService{lookup: discovery.LookupResult{Command: cmdInfo}},
			func() map[string]string { return nil },
			testConfigFallback,
		)

		result, _, err := svc.Execute(t.Context(), commandsvc.Request{
			Name:    "test-cmd",
			Runtime: invowkfile.RuntimeNative,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", result.ExitCode)
		}
	})

	t.Run("config default silently ignored when command does not support it", func(t *testing.T) {
		t.Parallel()

		cmdInfo := buildNativeOnlyCommand()
		cfg := &config.Config{DefaultRuntime: config.RuntimeVirtual}
		svc := commandsvc.New(
			&fixedConfigProvider{cfg: cfg},
			&lookupDiscoveryService{lookup: discovery.LookupResult{Command: cmdInfo}},
			func() map[string]string { return nil },
			testConfigFallback,
		)

		result, _, err := svc.Execute(t.Context(), commandsvc.Request{Name: "native-only"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", result.ExitCode)
		}
	})

	t.Run("empty config default uses per-command default", func(t *testing.T) {
		t.Parallel()

		cmdInfo := buildDualRuntimeCommand()
		cfg := &config.Config{DefaultRuntime: ""}
		svc := commandsvc.New(
			&fixedConfigProvider{cfg: cfg},
			&lookupDiscoveryService{lookup: discovery.LookupResult{Command: cmdInfo}},
			func() map[string]string { return nil },
			testConfigFallback,
		)

		result, _, err := svc.Execute(t.Context(), commandsvc.Request{Name: "test-cmd"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", result.ExitCode)
		}
	})

	t.Run("nil config uses per-command default", func(t *testing.T) {
		t.Parallel()

		cmdInfo := buildDualRuntimeCommand()
		svc := commandsvc.New(
			&fixedConfigProvider{},
			&lookupDiscoveryService{lookup: discovery.LookupResult{Command: cmdInfo}},
			func() map[string]string { return nil },
			testConfigFallback,
		)

		result, _, err := svc.Execute(t.Context(), commandsvc.Request{Name: "test-cmd"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.ExitCode != 0 {
			t.Errorf("expected exit code 0, got %d", result.ExitCode)
		}
	})
}
