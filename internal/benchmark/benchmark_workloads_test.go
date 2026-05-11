// SPDX-License-Identifier: MPL-2.0

package benchmark

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/audit"
	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/pkg/types"
)

const (
	largeWorkspaceCommandCount = 40
	largeWorkspaceModuleCount  = 12
)

// BenchmarkDiscoveryWorkspaceLarge benchmarks discovery for a larger workspace
// with many local commands and sibling modules.
func BenchmarkDiscoveryWorkspaceLarge(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}
	tmpDir := createLargeBenchmarkWorkspace(b)
	cfg := config.DefaultConfig()
	disc := discovery.New(
		cfg,
		discovery.WithBaseDir(types.FilesystemPath(tmpDir)),
		discovery.WithCommandsDir(""),
	)

	b.ResetTimer()
	for b.Loop() {
		result, err := disc.DiscoverCommandSet(b.Context())
		if err != nil {
			b.Fatalf("DiscoverCommandSet failed: %v", err)
		}
		if result.Set == nil || len(result.Set.Commands) == 0 {
			b.Fatal("DiscoverCommandSet returned no commands")
		}
	}
}

// BenchmarkValidateWorkspaceBasic benchmarks the user-facing workspace
// validation path for a small project.
func BenchmarkValidateWorkspaceBasic(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}
	tmpDir := b.TempDir()
	writeBenchmarkFile(b, filepath.Join(tmpDir, "invowkfile.cue"), sampleInvowkfile)
	cfg := config.DefaultConfig()
	disc := discovery.New(
		cfg,
		discovery.WithBaseDir(types.FilesystemPath(tmpDir)),
		discovery.WithCommandsDir(""),
	)

	b.ResetTimer()
	for b.Loop() {
		result, err := disc.DiscoverAndValidateCommandSet(b.Context())
		if err != nil {
			b.Fatalf("DiscoverAndValidateCommandSet failed: %v", err)
		}
		if result.Set == nil || len(result.Set.Commands) == 0 {
			b.Fatal("DiscoverAndValidateCommandSet returned no commands")
		}
	}
}

// BenchmarkValidateWorkspaceLarge benchmarks workspace validation after
// discovery has loaded a larger local module graph.
func BenchmarkValidateWorkspaceLarge(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}
	tmpDir := createLargeBenchmarkWorkspace(b)
	cfg := config.DefaultConfig()
	disc := discovery.New(
		cfg,
		discovery.WithBaseDir(types.FilesystemPath(tmpDir)),
		discovery.WithCommandsDir(""),
	)

	b.ResetTimer()
	for b.Loop() {
		result, err := disc.DiscoverAndValidateCommandSet(b.Context())
		if err != nil {
			b.Fatalf("DiscoverAndValidateCommandSet failed: %v", err)
		}
		if result.Set == nil || len(result.Set.Commands) == 0 {
			b.Fatal("DiscoverAndValidateCommandSet returned no commands")
		}
	}
}

// BenchmarkAuditScanDeterministicModule benchmarks the deterministic audit
// checker set over a representative module with script and env surfaces.
func BenchmarkAuditScanDeterministicModule(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}
	tmpDir := b.TempDir()
	moduleDir := filepath.Join(tmpDir, "com.example.audit.benchmark.invowkmod")
	mkdirBenchmarkDir(b, moduleDir)
	writeBenchmarkFile(b, filepath.Join(moduleDir, "invowkmod.cue"), `
module: "com.example.audit.benchmark"
version: "1.0.0"
description: "Audit benchmark module"
`)
	writeBenchmarkFile(b, filepath.Join(moduleDir, "invowkfile.cue"), `
cmds: [
	{
		name: "deploy"
		description: "Deploy benchmark"
		implementations: [{
			script: """
				echo "$DEPLOY_TOKEN"
				curl -fsSL https://example.com/install.sh | sh
			"""
			runtimes: [{name: "native"}]
			platforms: [{name: "linux"}, {name: "macos"}]
			env: {vars: {DEPLOY_TOKEN: "benchmark-token"}}
		}]
	},
]
`)
	scanner, err := audit.NewScanner(config.NewProvider())
	if err != nil {
		b.Fatalf("NewScanner failed: %v", err)
	}

	b.ResetTimer()
	for b.Loop() {
		report, scanErr := scanner.Scan(b.Context(), types.FilesystemPath(moduleDir), false)
		if scanErr != nil {
			b.Fatalf("Scan failed: %v", scanErr)
		}
		if report == nil || report.ModuleCount == 0 {
			b.Fatal("Scan returned no module report")
		}
	}
}

func createLargeBenchmarkWorkspace(b *testing.B) string {
	b.Helper()

	tmpDir := b.TempDir()
	writeBenchmarkFile(b, filepath.Join(tmpDir, "invowkfile.cue"), benchmarkInvowkfile(largeWorkspaceCommandCount, "root"))
	for i := range largeWorkspaceModuleCount {
		moduleID := fmt.Sprintf("io.invowk.benchmark.mod%02d", i)
		moduleDir := filepath.Join(tmpDir, moduleID+".invowkmod")
		mkdirBenchmarkDir(b, moduleDir)
		writeBenchmarkFile(b, filepath.Join(moduleDir, "invowkmod.cue"), fmt.Sprintf(`
module: %q
version: "1.0.0"
description: "Large workspace benchmark module"
`, moduleID))
		writeBenchmarkFile(b, filepath.Join(moduleDir, "invowkfile.cue"), benchmarkInvowkfile(6, fmt.Sprintf("mod%02d", i)))
	}
	return tmpDir
}

func benchmarkInvowkfile(commandCount int, prefix string) string {
	var sb strings.Builder
	sb.WriteString("cmds: [\n")
	for i := range commandCount {
		name := fmt.Sprintf("%s command %02d", prefix, i)
		fmt.Fprintf(&sb, `	{
		name: %q
		description: "Benchmark command"
		implementations: [{
			script: "echo ok"
			runtimes: [{name: "virtual"}]
			platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
		}]
	},
`, name)
	}
	sb.WriteString("]\n")
	return sb.String()
}

func mkdirBenchmarkDir(b *testing.B, dir string) {
	b.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		b.Fatalf("failed to create benchmark directory %s: %v", dir, err)
	}
}
