// SPDX-License-Identifier: MPL-2.0

package modulesync

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/app/modulecache"
	"github.com/invowk/invowk/pkg/types"
)

// BenchmarkModuleSyncExplicitDeps benchmarks the explicit dependency sync path
// with a deterministic injected fetcher.
func BenchmarkModuleSyncExplicitDeps(b *testing.B) {
	workDir := b.TempDir()
	cacheDir := b.TempDir()
	repoDir := b.TempDir()
	moduleDir := filepath.Join(repoDir, "tools.invowkmod")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		b.Fatalf("MkdirAll() error = %v", err)
	}
	writeBenchmarkModuleFile(b, filepath.Join(moduleDir, "invowkmod.cue"), `module: "io.example.tools"
version: "1.2.3"
description: "Benchmark tools"
`)
	writeBenchmarkModuleFile(b, filepath.Join(moduleDir, "invowkfile.cue"), `cmds: [{
	name: "build"
	description: "Build"
	implementations: [{
		script: "echo build"
		runtimes: [{name: "virtual"}]
		platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
	}]
}]
`)

	fetcher := &fakeModuleFetcher{
		repoPath:     types.FilesystemPath(repoDir),
		listVersions: []SemVer{"1.2.3"},
	}
	resolver, err := newResolverWithFetcher(types.FilesystemPath(workDir), types.FilesystemPath(cacheDir), fetcher)
	if err != nil {
		b.Fatalf("newResolverWithFetcher() error = %v", err)
	}
	requirements := []ModuleRef{{
		GitURL:  "https://github.com/example/tools.invowkmod.git",
		Version: "^1.0.0",
	}}
	cachePath := resolver.getCachePath(string(requirements[0].GitURL), "1.2.3", "")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		b.Fatalf("MkdirAll(cache parent) error = %v", err)
	}
	if err := modulecache.CopyModuleDir(types.FilesystemPath(moduleDir), types.FilesystemPath(cachePath)); err != nil {
		b.Fatalf("CopyModuleDir() error = %v", err)
	}

	b.ResetTimer()
	for b.Loop() {
		resolved, syncErr := resolver.Sync(b.Context(), requirements)
		if syncErr != nil {
			b.Fatalf("Sync() error = %v", syncErr)
		}
		if len(resolved) != 1 {
			b.Fatalf("Sync() resolved %d modules, want 1", len(resolved))
		}
	}
}

// BenchmarkModuleTidyTransitiveDeps benchmarks fixed-point transitive
// dependency discovery for the explicit-only module model.
func BenchmarkModuleTidyTransitiveDeps(b *testing.B) {
	refA := ModuleRef{GitURL: "https://github.com/org/A.git", Version: "^1.0.0"}
	refB := ModuleRef{GitURL: "https://github.com/org/B.git", Version: "^1.0.0"}
	refC := ModuleRef{GitURL: "https://github.com/org/C.git", Version: "^1.0.0"}
	refD := ModuleRef{GitURL: "https://github.com/org/D.git", Version: "^1.0.0"}
	graph := map[ModuleRefKey][]ModuleRef{
		refA.Key(): {refB, refC},
		refB.Key(): {refD},
		refC.Key(): {refD},
	}
	resolveAll := func(_ context.Context, requirements []ModuleRef, _ map[ModuleRefKey]ContentHash) ([]*ResolvedModule, error) {
		resolved := make([]*ResolvedModule, 0, len(requirements))
		for _, req := range requirements {
			resolved = append(resolved, &ResolvedModule{
				ModuleRef:      req,
				TransitiveDeps: graph[req.Key()],
			})
		}
		return resolved, nil
	}

	b.ResetTimer()
	for b.Loop() {
		missing, err := tidyToFixedPoint(b.Context(), []ModuleRef{refA}, nil, resolveAll)
		if err != nil {
			b.Fatalf("tidyToFixedPoint() error = %v", err)
		}
		if len(missing) != 3 {
			b.Fatalf("tidyToFixedPoint() missing %d modules, want 3", len(missing))
		}
	}
}

func writeBenchmarkModuleFile(b *testing.B, path, content string) {
	b.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}
