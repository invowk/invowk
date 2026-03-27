// SPDX-License-Identifier: MPL-2.0

package benchmark

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/discovery"
	"github.com/invowk/invowk/internal/runtime"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

const (
	// sampleInvowkfile is a representative invowkfile.cue for benchmarking CUE parsing.
	// It includes multiple commands with various features to exercise the parser.
	sampleInvowkfile = `
cmds: [
	{
		name: "build"
		description: "Build the project"
		implementations: [
			{
				script: "echo building..."
				runtimes: [{name: "native"}, {name: "virtual"}]
				platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
			},
		]
		flags: [
			{name: "verbose", description: "Enable verbose output", type: "bool", default_value: "false"},
			{name: "output", description: "Output directory", default_value: "./dist"},
		]
		args: [
			{name: "target", description: "Build target", required: true},
		]
	},
		{
			name: "test unit"
			description: "Run unit tests"
			implementations: [
				{
					script: "echo testing..."
					runtimes: [{name: "native"}, {name: "virtual"}]
					platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
				},
			]
			depends_on: {
				tools: [{alternatives: ["go"]}]
		}
	},
	{
		name: "test integration"
		description: "Run integration tests"
		implementations: [
			{
				script: "echo integration testing..."
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
			},
		]
		depends_on: {
			tools: [{alternatives: ["go"]}]
			capabilities: [{alternatives: ["containers"]}]
		}
	},
	{
		name: "deploy"
		description: "Deploy the application"
		implementations: [
			{
				script: "echo deploying..."
				runtimes: [{name: "native"}]
				platforms: [{name: "linux"}, {name: "macos"}]
				env: {
					vars: {DEPLOY_ENV: "production"}
				}
			},
		]
		flags: [
			{name: "env", description: "Target environment", required: true, validation: "^(dev|staging|prod)$"},
			{name: "dry-run", description: "Perform dry run", type: "bool", default_value: "false"},
		]
	},
		{
			name: "clean"
			description: "Clean build artifacts"
			implementations: [
				{
					script: "echo cleaning..."
					runtimes: [{name: "native"}, {name: "virtual"}]
					platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
				},
			]
		},
	]

env: {
	vars: {
		PROJECT_NAME: "benchmark-test"
		VERSION: "1.0.0"
	}
}

depends_on: {
	tools: [{alternatives: ["git"]}]
}
`

	// sampleInvowkmod is a representative invowkmod.cue for benchmarking module parsing.
	sampleInvowkmod = `
module: "io.invowk.benchmark"
version: "1.0.0"
description: "Benchmark test module for PGO profiling"
`

	// complexInvowkfile is a more complex invowkfile for stress-testing the parser.
	complexInvowkfile = `
cmds: [
	{
		name: "cmd1"
		description: "Command 1"
		implementations: [{script: "echo 1", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
	},
	{
		name: "cmd2"
		description: "Command 2"
		implementations: [{script: "echo 2", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
	},
	{
		name: "cmd3"
		description: "Command 3"
		implementations: [{script: "echo 3", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
	},
	{
		name: "cmd4"
		description: "Command 4"
		implementations: [{script: "echo 4", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
	},
	{
		name: "cmd5"
		description: "Command 5"
		implementations: [{script: "echo 5", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
	},
	{
		name: "cmd6"
		description: "Command 6"
		implementations: [{script: "echo 6", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
	},
	{
		name: "cmd7"
		description: "Command 7"
		implementations: [{script: "echo 7", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
	},
	{
		name: "cmd8"
		description: "Command 8"
		implementations: [{script: "echo 8", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
	},
	{
		name: "nested cmd1"
		description: "Nested command 1"
		implementations: [{script: "echo nested1", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
	},
	{
		name: "nested cmd2"
		description: "Nested command 2"
		implementations: [{script: "echo nested2", runtimes: [{name: "native"}], platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]}]
	},
]
	`
)

func writeBenchmarkFile(b *testing.B, path, content string) {
	b.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		b.Fatalf("failed to write %s: %v", path, err)
	}
}

func createBenchmarkModule(b *testing.B, baseDir, folderName, moduleID string) types.FilesystemPath {
	b.Helper()

	moduleDir := filepath.Join(baseDir, folderName+invowkmod.ModuleSuffix)
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		b.Fatalf("failed to create module directory %s: %v", moduleDir, err)
	}

	invowkmodContent := fmt.Sprintf(
		"module: %q\nversion: \"1.0.0\"\ndescription: \"benchmark module %s\"\n",
		moduleID,
		moduleID,
	)
	writeBenchmarkFile(b, filepath.Join(moduleDir, "invowkmod.cue"), invowkmodContent)
	writeBenchmarkFile(b, filepath.Join(moduleDir, "invowkfile.cue"), sampleInvowkfile)

	return types.FilesystemPath(moduleDir)
}

// setBenchmarkRuntime selects a runtime and matching implementation for the current host platform.
func setBenchmarkRuntime(b *testing.B, ctx *runtime.ExecutionContext, mode invowkfile.RuntimeMode) {
	b.Helper()

	platform := invowkfile.CurrentPlatform()
	ctx.SelectedRuntime = mode
	ctx.SelectedImpl = ctx.Command.GetImplForPlatformRuntime(platform, mode)
	if ctx.SelectedImpl == nil {
		b.Fatalf("no implementation available for runtime %q on platform %q", mode, platform)
	}
}

// BenchmarkCUEParsing benchmarks CUE schema compilation and validation.
// This exercises the hot path in pkg/cueutil/parse.go.
func BenchmarkCUEParsing(b *testing.B) {
	data := []byte(sampleInvowkfile)

	b.ResetTimer()
	for b.Loop() {
		_, err := invowkfile.ParseBytes(data, "benchmark.cue")
		if err != nil {
			b.Fatalf("ParseBytes failed: %v", err)
		}
	}
}

// BenchmarkCUEParsingComplex benchmarks parsing a larger invowkfile.
func BenchmarkCUEParsingComplex(b *testing.B) {
	data := []byte(complexInvowkfile)

	b.ResetTimer()
	for b.Loop() {
		_, err := invowkfile.ParseBytes(data, "complex.cue")
		if err != nil {
			b.Fatalf("ParseBytes failed: %v", err)
		}
	}
}

// BenchmarkInvowkmodParsing benchmarks module metadata parsing.
func BenchmarkInvowkmodParsing(b *testing.B) {
	data := []byte(sampleInvowkmod)

	b.ResetTimer()
	for b.Loop() {
		_, err := invowkmod.ParseInvowkmodBytes(data, "invowkmod.cue")
		if err != nil {
			b.Fatalf("ParseInvowkmodBytes failed: %v", err)
		}
	}
}

// BenchmarkDiscovery benchmarks module and command discovery.
// This exercises the hot path in internal/discovery/.
func BenchmarkDiscovery(b *testing.B) {
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
	tmpDir := b.TempDir()
	writeBenchmarkFile(b, filepath.Join(tmpDir, "invowkfile.cue"), sampleInvowkfile)

	parentModulePath := createBenchmarkModule(b, tmpDir, "parent", "parent")

	vendorRoot := filepath.Join(string(parentModulePath), invowkmod.VendoredModulesDir)
	if err := os.MkdirAll(vendorRoot, 0o755); err != nil {
		b.Fatalf("failed to create vendored modules directory: %v", err)
	}
	vendoredPath := createBenchmarkModule(b, vendorRoot, "child", "child")
	nestedVendorRoot := filepath.Join(string(vendoredPath), invowkmod.VendoredModulesDir)
	_ = createBenchmarkModule(b, nestedVendorRoot, "grandchild", "grandchild")

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

// BenchmarkRuntimeNative benchmarks native shell execution.
// This exercises the hot path in internal/runtime/native.go.
func BenchmarkRuntimeNative(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	tmpDir := b.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	// Create a minimal invowkfile for the test
	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
		Commands: []invowkfile.Command{
			{
				Name:        "test",
				Description: "Test command",
				Implementations: []invowkfile.Implementation{
					{
						Script: "echo hello",
						Runtimes: []invowkfile.RuntimeConfig{
							{Name: invowkfile.RuntimeNative},
						},
						Platforms: invowkfile.AllPlatformConfigs(),
					},
				},
			},
		},
	}

	cmd := inv.GetCommand("test")
	ctx := runtime.NewExecutionContext(b.Context(), cmd, inv)
	setBenchmarkRuntime(b, ctx, invowkfile.RuntimeNative)

	ctx.IO = runtime.IOContext{
		Stdout: io.Discard,
		Stderr: io.Discard,
		Stdin:  bytes.NewReader(nil),
	}

	rt := runtime.NewNativeRuntime()

	b.ResetTimer()
	for b.Loop() {
		result := rt.Execute(ctx)
		if result.Error != nil {
			b.Fatalf("Execute failed: %v", result.Error)
		}
	}
}

// BenchmarkRuntimeVirtual benchmarks mvdan/sh virtual shell execution.
// This exercises the hot path in internal/runtime/virtual.go.
func BenchmarkRuntimeVirtual(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	tmpDir := b.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
		Commands: []invowkfile.Command{
			{
				Name:        "test",
				Description: "Test command",
				Implementations: []invowkfile.Implementation{
					{
						Script: "echo hello",
						Runtimes: []invowkfile.RuntimeConfig{
							{Name: invowkfile.RuntimeVirtual},
						},
						Platforms: invowkfile.AllPlatformConfigs(),
					},
				},
			},
		},
	}

	cmd := inv.GetCommand("test")
	ctx := runtime.NewExecutionContext(b.Context(), cmd, inv)

	setBenchmarkRuntime(b, ctx, invowkfile.RuntimeVirtual)
	ctx.IO = runtime.IOContext{
		Stdout: io.Discard,
		Stderr: io.Discard,
		Stdin:  bytes.NewReader(nil),
	}

	rt := runtime.NewVirtualRuntime(true) // Enable u-root utilities

	b.ResetTimer()
	for b.Loop() {
		result := rt.Execute(ctx)
		if result.Error != nil {
			b.Fatalf("Execute failed: %v", result.Error)
		}
	}
}

// BenchmarkRuntimeVirtualComplex benchmarks virtual shell with more complex scripts.
func BenchmarkRuntimeVirtualComplex(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	tmpDir := b.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	// A more complex script that exercises more of the virtual shell
	script := `
VAR1="hello"
VAR2="world"
echo "$VAR1 $VAR2"
for i in 1 2 3; do
  echo "iteration $i"
done
if [ "$VAR1" = "hello" ]; then
  echo "condition matched"
fi
`

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
		Commands: []invowkfile.Command{
			{
				Name:        "complex",
				Description: "Complex command",
				Implementations: []invowkfile.Implementation{
					{
						Script: invowkfile.ScriptContent(script),
						Runtimes: []invowkfile.RuntimeConfig{
							{Name: invowkfile.RuntimeVirtual},
						},
						Platforms: invowkfile.AllPlatformConfigs(),
					},
				},
			},
		},
	}

	cmd := inv.GetCommand("complex")
	ctx := runtime.NewExecutionContext(b.Context(), cmd, inv)

	setBenchmarkRuntime(b, ctx, invowkfile.RuntimeVirtual)
	ctx.IO = runtime.IOContext{
		Stdout: io.Discard,
		Stderr: io.Discard,
		Stdin:  bytes.NewReader(nil),
	}

	rt := runtime.NewVirtualRuntime(true)

	b.ResetTimer()
	for b.Loop() {
		result := rt.Execute(ctx)
		if result.Error != nil {
			b.Fatalf("Execute failed: %v", result.Error)
		}
	}
}

// BenchmarkRuntimeContainer benchmarks container runtime execution.
// This test is skipped in short mode as it requires Docker/Podman.
func BenchmarkRuntimeContainer(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping container benchmark in short mode")
	}

	cfg := config.DefaultConfig()

	tmpDir := b.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
		Commands: []invowkfile.Command{
			{
				Name:        "container-test",
				Description: "Container test command",
				Implementations: []invowkfile.Implementation{
					{
						Script: "echo hello from container",
						Runtimes: []invowkfile.RuntimeConfig{
							{
								Name:  invowkfile.RuntimeContainer,
								Image: "debian:stable-slim",
							},
						},
						Platforms: invowkfile.AllPlatformConfigs(),
					},
				},
			},
		},
	}

	cmd := inv.GetCommand("container-test")
	ctx := runtime.NewExecutionContext(b.Context(), cmd, inv)

	setBenchmarkRuntime(b, ctx, invowkfile.RuntimeContainer)
	ctx.IO = runtime.IOContext{
		Stdout: io.Discard,
		Stderr: io.Discard,
		Stdin:  bytes.NewReader(nil),
	}

	rt, err := runtime.NewContainerRuntime(cfg)
	if err != nil {
		b.Skipf("skipping container benchmark: %v", err)
	}
	b.Cleanup(func() {
		_ = rt.Close()
	})
	if !rt.Available() {
		b.Skip("skipping container benchmark: container runtime is unavailable")
	}

	b.ResetTimer()
	for b.Loop() {
		result := rt.Execute(ctx)
		if result.Error != nil {
			b.Fatalf("Execute failed: %v", result.Error)
		}
	}
}

// BenchmarkFullPipeline benchmarks the end-to-end command execution pipeline.
// This exercises discovery, parsing, and execution together.
func BenchmarkFullPipeline(b *testing.B) {
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	tmpDir := b.TempDir()

	// Create invowkfile.cue
	invowkfileContent := `
cmds: [
	{
		name: "hello"
		description: "Say hello"
		implementations: [
			{
				script: "echo hello"
				runtimes: [{name: "virtual"}]
				platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
			},
		]
	},
]
`
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte(invowkfileContent), 0o644); err != nil {
		b.Fatalf("Failed to write invowkfile: %v", err)
	}

	// Change to temp dir
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
	rt := runtime.NewVirtualRuntime(true)

	b.ResetTimer()
	for b.Loop() {
		// Discovery phase
		disc := discovery.New(cfg)
		files, err := disc.LoadAll()
		if err != nil {
			b.Fatalf("LoadAll failed: %v", err)
		}

		if len(files) == 0 || files[0].Invowkfile == nil {
			b.Fatal("No invowkfile found")
		}

		inv := files[0].Invowkfile
		cmd := inv.GetCommand("hello")
		if cmd == nil {
			b.Fatal("Command 'hello' not found")
		}

		// Execution phase
		ctx := runtime.NewExecutionContext(b.Context(), cmd, inv)

		setBenchmarkRuntime(b, ctx, invowkfile.RuntimeVirtual)
		ctx.IO = runtime.IOContext{
			Stdout: io.Discard,
			Stderr: io.Discard,
			Stdin:  bytes.NewReader(nil),
		}

		result := rt.Execute(ctx)
		if result.Error != nil {
			b.Fatalf("Execute failed: %v", result.Error)
		}
	}
}

// BenchmarkCommandLookup benchmarks command lookup by name.
func BenchmarkCommandLookup(b *testing.B) {
	// Parse the complex invowkfile once
	data := []byte(complexInvowkfile)
	inv, err := invowkfile.ParseBytes(data, "complex.cue")
	if err != nil {
		b.Fatalf("ParseBytes failed: %v", err)
	}

	commandNames := []invowkfile.CommandName{"cmd1", "cmd5", "cmd8", "nested cmd1", "nested cmd2"}

	b.ResetTimer()
	for b.Loop() {
		for _, name := range commandNames {
			cmd := inv.GetCommand(name)
			if cmd == nil {
				b.Fatalf("Command %q not found", name)
			}
		}
	}
}

// BenchmarkEnvBuilding benchmarks environment variable building.
func BenchmarkEnvBuilding(b *testing.B) {
	tmpDir := b.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	inv := &invowkfile.Invowkfile{
		FilePath: invowkfile.FilesystemPath(invowkfilePath),
		Env: &invowkfile.EnvConfig{
			Vars: map[invowkfile.EnvVarName]string{
				"ROOT_VAR1": "value1",
				"ROOT_VAR2": "value2",
			},
		},
		Commands: []invowkfile.Command{
			{
				Name:        "test",
				Description: "Test command",
				Env: &invowkfile.EnvConfig{
					Vars: map[invowkfile.EnvVarName]string{
						"CMD_VAR1": "cmd_value1",
						"CMD_VAR2": "cmd_value2",
					},
				},
				Implementations: []invowkfile.Implementation{
					{
						Script: "echo test",
						Runtimes: []invowkfile.RuntimeConfig{
							{Name: invowkfile.RuntimeNative},
						},
						Platforms: invowkfile.AllPlatformConfigs(),
						Env: &invowkfile.EnvConfig{
							Vars: map[invowkfile.EnvVarName]string{
								"IMPL_VAR1": "impl_value1",
							},
						},
					},
				},
			},
		},
	}

	cmd := inv.GetCommand("test")
	ctx := runtime.NewExecutionContext(b.Context(), cmd, inv)

	envBuilder := runtime.NewDefaultEnvBuilder()

	b.ResetTimer()
	for b.Loop() {
		_, err := envBuilder.Build(ctx, invowkfile.EnvInheritAll)
		if err != nil {
			b.Fatalf("Build failed: %v", err)
		}
	}
}

// BenchmarkModuleValidation benchmarks module validation.
func BenchmarkModuleValidation(b *testing.B) {
	// Create a complete module structure
	// Note: The module name in invowkmod.cue must match the folder name (minus .invowkmod suffix)
	tmpDir := b.TempDir()
	modDir := filepath.Join(tmpDir, "benchmark.invowkmod")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		b.Fatalf("Failed to create module dir: %v", err)
	}

	// Create invowkmod.cue with module name matching folder
	invowkmodContent := `
module: "benchmark"
version: "1.0.0"
description: "Benchmark test module for PGO profiling"
`
	if err := os.WriteFile(filepath.Join(modDir, "invowkmod.cue"), []byte(invowkmodContent), 0o644); err != nil {
		b.Fatalf("Failed to write invowkmod.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "invowkfile.cue"), []byte(sampleInvowkfile), 0o644); err != nil {
		b.Fatalf("Failed to write invowkfile.cue: %v", err)
	}

	b.ResetTimer()
	for b.Loop() {
		result, err := invowkmod.Validate(types.FilesystemPath(modDir))
		if err != nil {
			b.Fatalf("Module validation error: %v", err)
		}
		if !result.Valid {
			b.Fatalf("Module validation failed: %v", result.Issues)
		}
	}
}
