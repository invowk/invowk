// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/app/commandadapters"
	"github.com/invowk/invowk/internal/app/commandsvc"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

type countingFailingConfigProvider struct {
	err   error
	calls int
}

func (p *countingFailingConfigProvider) Load(context.Context, config.LoadOptions) (*config.Config, error) {
	p.calls++
	return nil, p.err
}

func (p *countingFailingConfigProvider) LoadWithSource(ctx context.Context, opts config.LoadOptions) (config.LoadResult, error) {
	cfg, err := p.Load(ctx, opts)
	if err != nil {
		return config.LoadResult{}, err
	}
	return config.LoadResult{Config: cfg}, nil
}

// testConfigFallback wraps the application-service fallback for commandsvc.ConfigFallbackFunc.
func testConfigFallback(ctx context.Context, provider config.Loader, configPath string) (cfg *config.Config, diags []commandsvc.Diagnostic) {
	return commandsvc.LoadConfigWithFallback(ctx, provider, configPath)
}

func testRuntimeRegistryFactory(t testing.TB) commandsvc.RuntimeRegistryCreator {
	t.Helper()

	registryFactory, err := commandadapters.NewRuntimeRegistryFactory()
	if err != nil {
		t.Fatalf("NewRuntimeRegistryFactory() error = %v", err)
	}
	return registryFactory
}

func TestResolveCommandLoadsConfigOnceAndReportsOneDiagnostic(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "missing-config.cue")
	writeRuntimeTestInvowkfile(t, tmpDir)
	provider := &countingFailingConfigProvider{err: os.ErrNotExist}
	discoverySvc, err := commandadapters.NewDiscoveryServiceWithDirs(
		provider,
		types.FilesystemPath(tmpDir),
		"",
	)
	if err != nil {
		t.Fatalf("NewDiscoveryServiceWithDirs() error = %v", err)
	}
	app, err := NewApp(Dependencies{
		Config:    provider,
		Discovery: discoverySvc,
		Stdout:    io.Discard,
		Stderr:    io.Discard,
	})
	if err != nil {
		t.Fatalf("NewApp() error = %v", err)
	}

	_, _, diags, err := app.Commands.ResolveCommand(t.Context(), ExecuteRequest{
		Name:       "build",
		ConfigPath: types.FilesystemPath(configPath),
	})
	if err != nil {
		t.Fatalf("ResolveCommand() error = %v", err)
	}
	if provider.calls != 1 {
		t.Fatalf("config load calls = %d, want 1", provider.calls)
	}
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %d, want 1: %#v", len(diags), diags)
	}
	if diags[0].Code() != discovery.DiagnosticCode(commandsvc.DiagnosticCodeConfigLoadFailed) {
		t.Fatalf("diagnostic code = %q, want %q", diags[0].Code(), commandsvc.DiagnosticCodeConfigLoadFailed)
	}
}

func writeRuntimeTestInvowkfile(t testing.TB, dir string) {
	t.Helper()

	content := `cmds: [{
	name: "build"
	description: "Build command"
	implementations: [{
		script: {content: "echo build"}
		runtimes: [{name: "virtual"}]
		platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
	}]
}]
`
	if err := os.WriteFile(filepath.Join(dir, "invowkfile.cue"), []byte(content), 0o644); err != nil {
		t.Fatalf("write invowkfile: %v", err)
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
					Script:    invowkfile.ImplementationScript{Content: "echo hello"},
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
					Script:    invowkfile.ImplementationScript{Content: "echo native"},
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
			commandsvc.NewPorts(nil, testRuntimeRegistryFactory(t), nil, nil, nil, nil, nil, nil, nil),
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
			commandsvc.NewPorts(nil, testRuntimeRegistryFactory(t), nil, nil, nil, nil, nil, nil, nil),
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
			commandsvc.NewPorts(nil, testRuntimeRegistryFactory(t), nil, nil, nil, nil, nil, nil, nil),
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
			commandsvc.NewPorts(nil, testRuntimeRegistryFactory(t), nil, nil, nil, nil, nil, nil, nil),
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
			commandsvc.NewPorts(nil, testRuntimeRegistryFactory(t), nil, nil, nil, nil, nil, nil, nil),
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
