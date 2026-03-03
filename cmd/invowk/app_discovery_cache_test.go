// SPDX-License-Identifier: MPL-2.0

package cmd

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/invowkfile"
)

type (
	staticConfigProvider struct {
		cfg *config.Config
	}

	countingConfigProvider struct {
		cfg   *config.Config
		calls int
	}
)

func (p *staticConfigProvider) Load(_ context.Context, _ config.LoadOptions) (*config.Config, error) {
	return p.cfg, nil
}

func (p *countingConfigProvider) Load(_ context.Context, _ config.LoadOptions) (*config.Config, error) {
	p.calls++
	return p.cfg, nil
}

// Not parallel: os.Chdir is process-wide.
func TestAppDiscoveryService_RequestScopedCache_ReusesLookupResult(t *testing.T) {
	tmpDir := t.TempDir()
	invPath := filepath.Join(tmpDir, "invowkfile.cue")
	content := invowkfile.GenerateCUE(&invowkfile.Invowkfile{
		Commands: []invowkfile.Command{
			{
				Name: "build",
				Implementations: []invowkfile.Implementation{
					{
						Script:    "echo build",
						Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
						Platforms: invowkfile.AllPlatformConfigs(),
					},
				},
			},
		},
	})
	if err := os.WriteFile(invPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test invowkfile: %v", err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err = os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to test dir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	svc := &appDiscoveryService{
		config: &staticConfigProvider{cfg: config.DefaultConfig()},
	}

	ctx := contextWithConfigPath(t.Context(), "")

	first, err := svc.GetCommand(ctx, "build")
	if err != nil {
		t.Fatalf("first GetCommand() error: %v", err)
	}
	if first.Command == nil {
		t.Fatalf("first GetCommand() returned nil command (diagnostics: %#v)", first.Diagnostics)
	}

	second, err := svc.GetCommand(ctx, "build")
	if err != nil {
		t.Fatalf("second GetCommand() error: %v", err)
	}
	if second.Command == nil {
		t.Fatal("second GetCommand() returned nil command")
	}

	if first.Command != second.Command {
		t.Fatal("expected cached lookup to reuse command pointer within request scope")
	}
}

// Not parallel: os.Chdir is process-wide.
//
// Verifies the cross-population invariant: after DiscoverAndValidateCommandSet
// populates the cache, a subsequent DiscoverCommandSet call returns the cached
// result with nil error (even when tree validation failed). This ensures the
// listing path (DiscoverCommandSet) doesn't see tree validation errors as
// discovery failures.
func TestAppDiscoveryService_CrossPopulate_ValidatedSetPopulatesCommandSet(t *testing.T) {
	tmpDir := t.TempDir()
	invPath := filepath.Join(tmpDir, "invowkfile.cue")
	content := invowkfile.GenerateCUE(&invowkfile.Invowkfile{
		Commands: []invowkfile.Command{
			{
				Name: "build",
				Implementations: []invowkfile.Implementation{
					{
						Script:    "echo build",
						Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
						Platforms: invowkfile.AllPlatformConfigs(),
					},
				},
			},
		},
	})
	if err := os.WriteFile(invPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test invowkfile: %v", err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err = os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to test dir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	svc := &appDiscoveryService{
		config: &staticConfigProvider{cfg: config.DefaultConfig()},
	}

	ctx := contextWithConfigPath(t.Context(), "")

	// First call: DiscoverAndValidateCommandSet populates the cache.
	validated, validErr := svc.DiscoverAndValidateCommandSet(ctx)
	if validErr != nil {
		t.Fatalf("DiscoverAndValidateCommandSet() error: %v", validErr)
	}
	if validated.Set == nil {
		t.Fatal("DiscoverAndValidateCommandSet() returned nil Set")
	}

	// Second call: DiscoverCommandSet should return the cached result.
	discovered, discErr := svc.DiscoverCommandSet(ctx)
	if discErr != nil {
		t.Fatalf("DiscoverCommandSet() error after cross-population: %v", discErr)
	}
	if discovered.Set == nil {
		t.Fatal("DiscoverCommandSet() returned nil Set after cross-population")
	}

	// Verify the cross-populated Set has the same commands.
	if len(discovered.Set.Commands) != len(validated.Set.Commands) {
		t.Errorf("cross-populated command count = %d, want %d",
			len(discovered.Set.Commands), len(validated.Set.Commands))
	}
}

// Not parallel: os.Chdir is process-wide.
func TestAppDiscoveryService_WithoutCacheContext_DoesNotMemoizeLookup(t *testing.T) {
	tmpDir := t.TempDir()
	invPath := filepath.Join(tmpDir, "invowkfile.cue")
	content := invowkfile.GenerateCUE(&invowkfile.Invowkfile{
		Commands: []invowkfile.Command{
			{
				Name: "build",
				Implementations: []invowkfile.Implementation{
					{
						Script:    "echo build",
						Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
						Platforms: invowkfile.AllPlatformConfigs(),
					},
				},
			},
		},
	})
	if err := os.WriteFile(invPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test invowkfile: %v", err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err = os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to test dir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	svc := &appDiscoveryService{
		config: &staticConfigProvider{cfg: config.DefaultConfig()},
	}

	// Directly set config path in context to bypass contextWithConfigPath(), which
	// attaches request cache.
	ctx := context.WithValue(t.Context(), configPathContextKey{}, "")

	first, err := svc.GetCommand(ctx, "build")
	if err != nil {
		t.Fatalf("first GetCommand() error: %v", err)
	}
	if first.Command == nil {
		t.Fatalf("first GetCommand() returned nil command (diagnostics: %#v)", first.Diagnostics)
	}

	second, err := svc.GetCommand(ctx, "build")
	if err != nil {
		t.Fatalf("second GetCommand() error: %v", err)
	}
	if second.Command == nil {
		t.Fatal("second GetCommand() returned nil command")
	}

	if first.Command == second.Command {
		t.Fatal("expected uncached lookups to re-parse and return distinct command pointers")
	}
}

// Not parallel: os.Chdir is process-wide.
func TestAppDiscoveryService_RequestScopedConfigCache_ReusesConfigLoad(t *testing.T) {
	tmpDir := t.TempDir()
	invPath := filepath.Join(tmpDir, "invowkfile.cue")
	content := invowkfile.GenerateCUE(&invowkfile.Invowkfile{
		Commands: []invowkfile.Command{
			{
				Name: "build",
				Implementations: []invowkfile.Implementation{
					{
						Script:    "echo build",
						Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
						Platforms: invowkfile.AllPlatformConfigs(),
					},
				},
			},
		},
	})
	if err := os.WriteFile(invPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test invowkfile: %v", err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err = os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to test dir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	cfgProvider := &countingConfigProvider{cfg: config.DefaultConfig()}
	svc := &appDiscoveryService{config: cfgProvider}
	ctx := contextWithConfigPath(t.Context(), "")

	if _, err = svc.DiscoverCommandSet(ctx); err != nil {
		t.Fatalf("DiscoverCommandSet() error: %v", err)
	}
	if _, err = svc.DiscoverAndValidateCommandSet(ctx); err != nil {
		t.Fatalf("DiscoverAndValidateCommandSet() error: %v", err)
	}
	if _, err = svc.GetCommand(ctx, "build"); err != nil {
		t.Fatalf("GetCommand() error: %v", err)
	}

	if cfgProvider.calls != 1 {
		t.Fatalf("config provider Load() calls = %d, want 1", cfgProvider.calls)
	}
}

// Not parallel: os.Chdir is process-wide.
func TestAppDiscoveryService_GetCommand_UsesCachedCommandSetLookup(t *testing.T) {
	tmpDir := t.TempDir()
	invPath := filepath.Join(tmpDir, "invowkfile.cue")
	content := invowkfile.GenerateCUE(&invowkfile.Invowkfile{
		Commands: []invowkfile.Command{
			{
				Name: "build",
				Implementations: []invowkfile.Implementation{
					{
						Script:    "echo build",
						Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
						Platforms: invowkfile.AllPlatformConfigs(),
					},
				},
			},
		},
	})
	if err := os.WriteFile(invPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test invowkfile: %v", err)
	}

	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	if err = os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to chdir to test dir: %v", err)
	}
	defer func() { _ = os.Chdir(oldWD) }()

	svc := &appDiscoveryService{config: &staticConfigProvider{cfg: config.DefaultConfig()}}
	ctx := contextWithConfigPath(t.Context(), "")

	commandSetResult, err := svc.DiscoverAndValidateCommandSet(ctx)
	if err != nil {
		t.Fatalf("DiscoverAndValidateCommandSet() error: %v", err)
	}
	expected := commandSetResult.Set.ByName["build"]
	if expected == nil {
		t.Fatal("expected command set to include 'build'")
	}

	lookupResult, err := svc.GetCommand(ctx, "build")
	if err != nil {
		t.Fatalf("GetCommand() error: %v", err)
	}
	if lookupResult.Command == nil {
		t.Fatal("GetCommand() returned nil command")
	}
	if lookupResult.Command != expected {
		t.Fatal("expected GetCommand() to reuse cached command pointer from command set")
	}
}
