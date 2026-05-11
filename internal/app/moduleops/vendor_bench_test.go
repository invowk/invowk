// SPDX-License-Identifier: MPL-2.0

package moduleops

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

// BenchmarkModuleVendorLockedDeps benchmarks copying locked resolved modules
// from the local module cache into invowk_modules/.
func BenchmarkModuleVendorLockedDeps(b *testing.B) {
	tmpDir := b.TempDir()
	modulePath := createBenchmarkVendorModule(b, tmpDir, "parent.invowkmod", "parent")
	cache1 := createBenchmarkCacheModule(b, tmpDir, "dep1.invowkmod", "dep1")
	cache2 := createBenchmarkCacheModule(b, tmpDir, "dep2.invowkmod", "dep2")
	dep1Hash := benchmarkModuleHash(b, filepath.Join(cache1, "dep1.invowkmod"))
	dep2Hash := benchmarkModuleHash(b, filepath.Join(cache2, "dep2.invowkmod"))
	modules := []*invowkmod.ResolvedModule{
		{
			ModuleRef: invowkmod.ModuleRef{
				GitURL:  "https://github.com/example/dep1.invowkmod.git",
				Version: "1.0.0",
			},
			CachePath:   types.FilesystemPath(cache1),
			Namespace:   "dep1@1.0.0",
			ModuleID:    "dep1",
			ContentHash: dep1Hash,
		},
		{
			ModuleRef: invowkmod.ModuleRef{
				GitURL:  "https://github.com/example/dep2.invowkmod.git",
				Version: "2.0.0",
			},
			CachePath:   types.FilesystemPath(cache2),
			Namespace:   "dep2@2.0.0",
			ModuleID:    "dep2",
			ContentHash: dep2Hash,
		},
	}

	b.ResetTimer()
	for b.Loop() {
		result, err := VendorModules(VendorOptions{
			ModulePath: types.FilesystemPath(modulePath),
			Modules:    modules,
			Prune:      true,
		})
		if err != nil {
			b.Fatalf("VendorModules() error = %v", err)
		}
		if len(result.Vendored) != len(modules) {
			b.Fatalf("VendorModules() vendored %d modules, want %d", len(result.Vendored), len(modules))
		}
	}
}

func createBenchmarkCacheModule(b *testing.B, parentDir, folderName, moduleID string) string {
	b.Helper()
	cacheDir := filepath.Join(parentDir, "cache-"+moduleID)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		b.Fatalf("MkdirAll(%s) error = %v", cacheDir, err)
	}
	createBenchmarkVendorModule(b, cacheDir, folderName, moduleID)
	return cacheDir
}

func createBenchmarkVendorModule(b *testing.B, parentDir, folderName, moduleID string) string {
	b.Helper()
	modulePath := filepath.Join(parentDir, folderName)
	if err := os.MkdirAll(modulePath, 0o755); err != nil {
		b.Fatalf("MkdirAll(%s) error = %v", modulePath, err)
	}
	writeBenchmarkVendorFile(b, filepath.Join(modulePath, "invowkmod.cue"), `module: "`+moduleID+`"
version: "1.0.0"
description: "Benchmark module"
`)
	writeBenchmarkVendorFile(b, filepath.Join(modulePath, "invowkfile.cue"), `cmds: [{
	name: "build"
	description: "Build"
	implementations: [{
		script: "echo build"
		runtimes: [{name: "virtual"}]
		platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
	}]
}]
`)
	return modulePath
}

func writeBenchmarkVendorFile(b *testing.B, path, content string) {
	b.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func benchmarkModuleHash(b *testing.B, modulePath string) invowkmod.ContentHash {
	b.Helper()
	hash, err := invowkmod.ComputeModuleHash(modulePath)
	if err != nil {
		b.Fatalf("ComputeModuleHash(%s) error = %v", modulePath, err)
	}
	return hash
}
