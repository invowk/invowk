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

type staticConfigProvider struct {
	cfg *config.Config
}

func (p *staticConfigProvider) Load(_ context.Context, _ config.LoadOptions) (*config.Config, error) {
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

	ctx := contextWithConfigPath(context.Background(), "")

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
	ctx := context.WithValue(context.Background(), configPathContextKey{}, "")

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
