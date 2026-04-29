// SPDX-License-Identifier: MPL-2.0

package benchmark

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

// BenchmarkDiscovery benchmarks module and command discovery.
// This exercises the hot path in internal/discovery/.
func BenchmarkDiscovery(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}
	// Create a temp directory structure for discovery
	tmpDir := b.TempDir()

	// Create invowkfile.cue
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(sampleInvowkfile), 0o644); err != nil {
		b.Fatalf("Failed to write invowkfile: %v", err)
	}

	// Create a sample module
	modDir := filepath.Join(tmpDir, "io.invowk.benchmark.invowkmod")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		b.Fatalf("Failed to create module dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "invowkmod.cue"), []byte(sampleInvowkmod), 0o644); err != nil {
		b.Fatalf("Failed to write invowkmod.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "invowkfile.cue"), []byte(sampleInvowkfile), 0o644); err != nil {
		b.Fatalf("Failed to write module invowkfile.cue: %v", err)
	}

	// Change to temp dir for discovery
	origDir, err := os.Getwd()
	if err != nil {
		b.Fatalf("Failed to get working directory: %v", err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		b.Fatalf("Failed to change to temp dir: %v", err)
	}
	b.Cleanup(func() {
		_ = os.Chdir(origDir)
	})

	cfg := config.DefaultConfig()
	disc := discovery.New(cfg)

	b.ResetTimer()
	for b.Loop() {
		files, err := disc.LoadAll()
		if err != nil {
			b.Fatalf("LoadAll failed: %v", err)
		}
		if len(files) == 0 {
			b.Fatal("LoadAll returned no discovered files")
		}
		for _, file := range files {
			if file.Error != nil {
				b.Fatalf("discovered file contains parse error: %v", file.Error)
			}
			if file.Invowkfile == nil {
				b.Fatalf("discovered file %q has no parsed invowkfile", file.Path)
			}
		}
	}
}

// BenchmarkDiscoveryIncludesAndAliases benchmarks discovery with configured
// include entries and alias-based module ID disambiguation.
func BenchmarkDiscoveryIncludesAndAliases(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}
	tmpDir := b.TempDir()
	writeBenchmarkFile(b, filepath.Join(tmpDir, "invowkfile.cue"), sampleInvowkfile)

	includeRootAlpha := filepath.Join(tmpDir, "includes-alpha")
	includeRootBeta := filepath.Join(tmpDir, "includes-beta")
	if err := os.MkdirAll(includeRootAlpha, 0o755); err != nil {
		b.Fatalf("failed to create includes-alpha directory: %v", err)
	}
	if err := os.MkdirAll(includeRootBeta, 0o755); err != nil {
		b.Fatalf("failed to create includes-beta directory: %v", err)
	}

	moduleAPath := createBenchmarkModule(b, includeRootAlpha, "shared", "shared")
	moduleBPath := createBenchmarkModule(b, includeRootBeta, "shared", "shared")

	cfg := config.DefaultConfig()
	cfg.Includes = []config.IncludeEntry{
		{
			Path:  config.ModuleIncludePath(moduleAPath),
			Alias: invowkmod.ModuleAlias("alpha"),
		},
		{
			Path:  config.ModuleIncludePath(moduleBPath),
			Alias: invowkmod.ModuleAlias("beta"),
		},
	}
	disc := discovery.New(cfg, discovery.WithBaseDir(types.FilesystemPath(tmpDir)), discovery.WithCommandsDir(""))

	b.ResetTimer()
	for b.Loop() {
		files, err := disc.LoadAll()
		if err != nil {
			b.Fatalf("LoadAll failed: %v", err)
		}

		aliasHits := map[invowkmod.ModuleAlias]bool{
			"alpha": false,
			"beta":  false,
		}
		for _, file := range files {
			if file.Module == nil || file.Invowkfile == nil {
				continue
			}

			switch invowkmod.ModuleAlias(disc.GetEffectiveModuleID(file)) {
			case "alpha":
				aliasHits["alpha"] = true
			case "beta":
				aliasHits["beta"] = true
			}
		}
		if !aliasHits["alpha"] || !aliasHits["beta"] {
			b.Fatalf("expected both aliases to be applied, got alpha=%t beta=%t", aliasHits["alpha"], aliasHits["beta"])
		}
	}
}

// BenchmarkDiscoveryVendoredModules benchmarks one-level vendored module discovery.
func BenchmarkDiscoveryVendoredModules(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}
	tmpDir := b.TempDir()
	writeBenchmarkFile(b, filepath.Join(tmpDir, "invowkfile.cue"), sampleInvowkfile)

	parentModulePath := createBenchmarkModule(b, tmpDir, "parent", "parent")
	writeBenchmarkFile(b, filepath.Join(string(parentModulePath), "invowkmod.cue"), `
module: "parent"
version: "1.0.0"
description: "benchmark parent module"
requires: [
	{
		git_url: "https://github.com/example/child.invowkmod.git"
		version: "^1.0.0"
	},
]
`)

	vendorRoot := filepath.Join(string(parentModulePath), invowkmod.VendoredModulesDir)
	if err := os.MkdirAll(vendorRoot, 0o755); err != nil {
		b.Fatalf("failed to create vendored modules directory: %v", err)
	}
	vendoredPath := createBenchmarkModule(b, vendorRoot, "child", "child")
	nestedVendorRoot := filepath.Join(string(vendoredPath), invowkmod.VendoredModulesDir)
	_ = createBenchmarkModule(b, nestedVendorRoot, "grandchild", "grandchild")
	childHash, err := invowkmod.ComputeModuleHash(string(vendoredPath))
	if err != nil {
		b.Fatalf("failed to compute vendored module hash: %v", err)
	}
	writeBenchmarkFile(b, filepath.Join(string(parentModulePath), invowkmod.LockFileName), fmt.Sprintf(`
version: "2.0"
generated: "2026-04-29T00:00:00Z"

modules: {
	"https://github.com/example/child.invowkmod.git": {
		git_url:          "https://github.com/example/child.invowkmod.git"
		version:          "^1.0.0"
		resolved_version: "1.0.0"
		git_commit:       "abc123def456789012345678901234567890abcd"
		namespace:        "child@1.0.0"
		module_id:        "child"
		content_hash:     %q
	}
}
`, childHash))

	cfg := config.DefaultConfig()
	disc := discovery.New(cfg, discovery.WithBaseDir(types.FilesystemPath(tmpDir)), discovery.WithCommandsDir(""))

	b.ResetTimer()
	for b.Loop() {
		files, err := disc.LoadAll()
		if err != nil {
			b.Fatalf("LoadAll failed: %v", err)
		}

		var vendoredCount int
		for _, file := range files {
			if file.ParentModule != nil {
				vendoredCount++
			}
		}
		if vendoredCount == 0 {
			b.Fatal("expected at least one vendored module in discovery results")
		}
	}
}

// BenchmarkDiscoveryModuleCollisionCheck benchmarks collision detection when two
// modules declare the same module ID and no alias is configured.
func BenchmarkDiscoveryModuleCollisionCheck(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}
	tmpDir := b.TempDir()
	writeBenchmarkFile(b, filepath.Join(tmpDir, "invowkfile.cue"), sampleInvowkfile)

	cfg := config.DefaultConfig()
	includeRootOne := filepath.Join(tmpDir, "include-one")
	includeRootTwo := filepath.Join(tmpDir, "include-two")
	if err := os.MkdirAll(includeRootOne, 0o755); err != nil {
		b.Fatalf("failed to create include-one directory: %v", err)
	}
	if err := os.MkdirAll(includeRootTwo, 0o755); err != nil {
		b.Fatalf("failed to create include-two directory: %v", err)
	}

	moduleOne := createBenchmarkModule(b, includeRootOne, "shared", "shared")
	moduleTwo := createBenchmarkModule(b, includeRootTwo, "shared", "shared")
	cfg.Includes = []config.IncludeEntry{
		{Path: config.ModuleIncludePath(moduleOne)},
		{Path: config.ModuleIncludePath(moduleTwo)},
	}
	disc := discovery.New(cfg, discovery.WithBaseDir(types.FilesystemPath(tmpDir)), discovery.WithCommandsDir(""))

	b.ResetTimer()
	for b.Loop() {
		_, err := disc.LoadAll()
		if err == nil {
			b.Fatal("expected module collision error")
		}
		var collisionErr *discovery.ModuleCollisionError
		if !errors.As(err, &collisionErr) {
			b.Fatalf("expected ModuleCollisionError, got: %v", err)
		}
	}
}
