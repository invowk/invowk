// SPDX-License-Identifier: MPL-2.0

package benchmark

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

// TestSampleInvowkfileParseable verifies the benchmark's sampleInvowkfile constant
// produces a valid Invowkfile when parsed. A refactoring that corrupts this constant
// would silently invalidate all CUE parsing benchmarks.
func TestSampleInvowkfileParseable(t *testing.T) {
	t.Parallel()

	inv, err := invowkfile.ParseBytes([]byte(sampleInvowkfile), "benchmark.cue")
	if err != nil {
		t.Fatalf("sampleInvowkfile failed to parse: %v", err)
	}

	if len(inv.Commands) != 5 {
		t.Errorf("expected 5 commands in sampleInvowkfile, got %d", len(inv.Commands))
	}

	expectedNames := []invowkfile.CommandName{"build", "test unit", "test integration", "deploy", "clean"}
	for _, name := range expectedNames {
		if inv.GetCommand(name) == nil {
			t.Errorf("expected command %q in sampleInvowkfile", name)
		}
	}
}

// TestComplexInvowkfileParseable verifies the complexInvowkfile constant is valid.
func TestComplexInvowkfileParseable(t *testing.T) {
	t.Parallel()

	inv, err := invowkfile.ParseBytes([]byte(complexInvowkfile), "complex.cue")
	if err != nil {
		t.Fatalf("complexInvowkfile failed to parse: %v", err)
	}

	if len(inv.Commands) != 10 {
		t.Errorf("expected 10 commands in complexInvowkfile, got %d", len(inv.Commands))
	}
}

// TestSampleInvowkmodParseable verifies the sampleInvowkmod constant is valid.
func TestSampleInvowkmodParseable(t *testing.T) {
	t.Parallel()

	mod, err := invowkmod.ParseInvowkmodBytes([]byte(sampleInvowkmod), "invowkmod.cue")
	if err != nil {
		t.Fatalf("sampleInvowkmod failed to parse: %v", err)
	}

	if string(mod.Module) != "io.invowk.benchmark" {
		t.Errorf("expected module ID %q, got %q", "io.invowk.benchmark", mod.Module)
	}
}

// TestCreateBenchmarkModule verifies the module directory fixture helper produces
// a valid module structure that discovery can load.
func TestCreateBenchmarkModule(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// The benchmark helper createBenchmarkModule takes *testing.B, so its logic
	// is replicated here for *testing.T.
	folderName := "testmod"
	moduleID := "io.invowk.testmod"
	moduleDir := filepath.Join(tmpDir, folderName+invowkmod.ModuleSuffix)

	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("failed to create module directory: %v", err)
	}

	invowkmodContent := "module: \"" + moduleID + "\"\nversion: \"1.0.0\"\ndescription: \"test module\"\n"
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkmod.cue"), []byte(invowkmodContent), 0o644); err != nil {
		t.Fatalf("failed to write invowkmod.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkfile.cue"), []byte(sampleInvowkfile), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile.cue: %v", err)
	}

	// Verify the module is parseable
	mod, err := invowkmod.ParseInvowkmodBytes(
		mustReadFile(t, filepath.Join(moduleDir, "invowkmod.cue")),
		"invowkmod.cue",
	)
	if err != nil {
		t.Fatalf("created module's invowkmod.cue failed to parse: %v", err)
	}
	if string(mod.Module) != moduleID {
		t.Errorf("expected module ID %q, got %q", moduleID, mod.Module)
	}

	// Verify the invowkfile is parseable
	inv, err := invowkfile.ParseBytes(
		mustReadFile(t, filepath.Join(moduleDir, "invowkfile.cue")),
		"invowkfile.cue",
	)
	if err != nil {
		t.Fatalf("created module's invowkfile.cue failed to parse: %v", err)
	}
	if len(inv.Commands) == 0 {
		t.Error("expected at least one command in created module's invowkfile")
	}
}

// TestDiscoveryFixtureProducesResults verifies that a benchmark-style directory
// structure (invowkfile.cue + module) is loadable by the discovery system.
func TestDiscoveryFixtureProducesResults(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create invowkfile.cue in root
	if err := os.WriteFile(filepath.Join(tmpDir, "invowkfile.cue"), []byte(sampleInvowkfile), 0o644); err != nil {
		t.Fatalf("failed to write invowkfile.cue: %v", err)
	}

	// Create a module
	moduleDir := filepath.Join(tmpDir, "io.invowk.benchmark.invowkmod")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("failed to create module dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkmod.cue"), []byte(sampleInvowkmod), 0o644); err != nil {
		t.Fatalf("failed to write invowkmod.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkfile.cue"), []byte(sampleInvowkfile), 0o644); err != nil {
		t.Fatalf("failed to write module invowkfile.cue: %v", err)
	}

	cfg := config.DefaultConfig()
	disc := discovery.New(cfg, discovery.WithBaseDir(types.FilesystemPath(tmpDir)), discovery.WithCommandsDir(""))

	files, err := disc.LoadAll()
	if err != nil {
		t.Fatalf("discovery.LoadAll failed: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("expected at least one discovered file")
	}

	for _, file := range files {
		if file.Error != nil {
			t.Errorf("discovered file %q contains parse error: %v", file.Path, file.Error)
		}
		if file.Invowkfile == nil {
			t.Errorf("discovered file %q has no parsed invowkfile", file.Path)
		}
	}
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return data
}
