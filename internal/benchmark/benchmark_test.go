// SPDX-License-Identifier: MPL-2.0

package benchmark

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"invowk-cli/internal/config"
	"invowk-cli/internal/discovery"
	"invowk-cli/internal/runtime"
	"invowk-cli/pkg/invkfile"
	"invowk-cli/pkg/invkmod"
)

const (
	// sampleInvkfile is a representative invkfile.cue for benchmarking CUE parsing.
	// It includes multiple commands with various features to exercise the parser.
	sampleInvkfile = `
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

	// sampleInvkmod is a representative invkmod.cue for benchmarking module parsing.
	sampleInvkmod = `
module: "io.invowk.benchmark"
version: "1.0"
description: "Benchmark test module for PGO profiling"
`

	// complexInvkfile is a more complex invkfile for stress-testing the parser.
	complexInvkfile = `
cmds: [
	{
		name: "cmd1"
		description: "Command 1"
		implementations: [{script: "echo 1", runtimes: [{name: "native"}]}]
	},
	{
		name: "cmd2"
		description: "Command 2"
		implementations: [{script: "echo 2", runtimes: [{name: "native"}]}]
	},
	{
		name: "cmd3"
		description: "Command 3"
		implementations: [{script: "echo 3", runtimes: [{name: "native"}]}]
	},
	{
		name: "cmd4"
		description: "Command 4"
		implementations: [{script: "echo 4", runtimes: [{name: "native"}]}]
	},
	{
		name: "cmd5"
		description: "Command 5"
		implementations: [{script: "echo 5", runtimes: [{name: "native"}]}]
	},
	{
		name: "cmd6"
		description: "Command 6"
		implementations: [{script: "echo 6", runtimes: [{name: "native"}]}]
	},
	{
		name: "cmd7"
		description: "Command 7"
		implementations: [{script: "echo 7", runtimes: [{name: "native"}]}]
	},
	{
		name: "cmd8"
		description: "Command 8"
		implementations: [{script: "echo 8", runtimes: [{name: "native"}]}]
	},
	{
		name: "nested cmd1"
		description: "Nested command 1"
		implementations: [{script: "echo nested1", runtimes: [{name: "native"}]}]
	},
	{
		name: "nested cmd2"
		description: "Nested command 2"
		implementations: [{script: "echo nested2", runtimes: [{name: "native"}]}]
	},
]
`
)

// BenchmarkCUEParsing benchmarks CUE schema compilation and validation.
// This exercises the hot path in internal/cueutil/parse.go.
func BenchmarkCUEParsing(b *testing.B) {
	data := []byte(sampleInvkfile)

	b.ResetTimer()
	for b.Loop() {
		_, err := invkfile.ParseBytes(data, "benchmark.cue")
		if err != nil {
			b.Fatalf("ParseBytes failed: %v", err)
		}
	}
}

// BenchmarkCUEParsingComplex benchmarks parsing a larger invkfile.
func BenchmarkCUEParsingComplex(b *testing.B) {
	data := []byte(complexInvkfile)

	b.ResetTimer()
	for b.Loop() {
		_, err := invkfile.ParseBytes(data, "complex.cue")
		if err != nil {
			b.Fatalf("ParseBytes failed: %v", err)
		}
	}
}

// BenchmarkInvkmodParsing benchmarks module metadata parsing.
func BenchmarkInvkmodParsing(b *testing.B) {
	data := []byte(sampleInvkmod)

	b.ResetTimer()
	for b.Loop() {
		_, err := invkmod.ParseInvkmodBytes(data, "invkmod.cue")
		if err != nil {
			b.Fatalf("ParseInvkmodBytes failed: %v", err)
		}
	}
}

// BenchmarkDiscovery benchmarks module and command discovery.
// This exercises the hot path in internal/discovery/.
func BenchmarkDiscovery(b *testing.B) {
	// Create a temp directory structure for discovery
	tmpDir := b.TempDir()

	// Create invkfile.cue
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(sampleInvkfile), 0o644); err != nil {
		b.Fatalf("Failed to write invkfile: %v", err)
	}

	// Create a sample module
	modDir := filepath.Join(tmpDir, "sample.invkmod")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		b.Fatalf("Failed to create module dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "invkmod.cue"), []byte(sampleInvkmod), 0o644); err != nil {
		b.Fatalf("Failed to write invkmod.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "invkfile.cue"), []byte(sampleInvkfile), 0o644); err != nil {
		b.Fatalf("Failed to write module invkfile.cue: %v", err)
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
		_, err := disc.LoadAll()
		if err != nil {
			b.Fatalf("LoadAll failed: %v", err)
		}
	}
}

// BenchmarkRuntimeNative benchmarks native shell execution.
// This exercises the hot path in internal/runtime/native.go.
func BenchmarkRuntimeNative(b *testing.B) {
	tmpDir := b.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	// Create a minimal invkfile for the test
	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
		Commands: []invkfile.Command{
			{
				Name:        "test",
				Description: "Test command",
				Implementations: []invkfile.Implementation{
					{
						Script: "echo hello",
						Runtimes: []invkfile.RuntimeConfig{
							{Name: invkfile.RuntimeNative},
						},
					},
				},
			},
		},
	}

	cmd := inv.GetCommand("test")
	ctx := runtime.NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()
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
	tmpDir := b.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
		Commands: []invkfile.Command{
			{
				Name:        "test",
				Description: "Test command",
				Implementations: []invkfile.Implementation{
					{
						Script: "echo hello",
						Runtimes: []invkfile.RuntimeConfig{
							{Name: invkfile.RuntimeVirtual},
						},
					},
				},
			},
		},
	}

	cmd := inv.GetCommand("test")
	ctx := runtime.NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()
	ctx.SelectedRuntime = invkfile.RuntimeVirtual
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
	tmpDir := b.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

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

	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
		Commands: []invkfile.Command{
			{
				Name:        "complex",
				Description: "Complex command",
				Implementations: []invkfile.Implementation{
					{
						Script: script,
						Runtimes: []invkfile.RuntimeConfig{
							{Name: invkfile.RuntimeVirtual},
						},
					},
				},
			},
		},
	}

	cmd := inv.GetCommand("complex")
	ctx := runtime.NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()
	ctx.SelectedRuntime = invkfile.RuntimeVirtual
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

	// Check if container engine is available
	cfg := config.DefaultConfig()
	if cfg.ContainerEngine == "" {
		b.Skip("no container engine available")
	}

	tmpDir := b.TempDir()
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
		Commands: []invkfile.Command{
			{
				Name:        "container-test",
				Description: "Container test command",
				Implementations: []invkfile.Implementation{
					{
						Script: "echo hello from container",
						Runtimes: []invkfile.RuntimeConfig{
							{
								Name:  invkfile.RuntimeContainer,
								Image: "debian:stable-slim",
							},
						},
					},
				},
			},
		},
	}

	cmd := inv.GetCommand("container-test")
	ctx := runtime.NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()
	ctx.SelectedRuntime = invkfile.RuntimeContainer
	ctx.IO = runtime.IOContext{
		Stdout: io.Discard,
		Stderr: io.Discard,
		Stdin:  bytes.NewReader(nil),
	}

	rt, err := runtime.NewContainerRuntime(cfg)
	if err != nil {
		b.Fatalf("NewContainerRuntime failed: %v", err)
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
	tmpDir := b.TempDir()

	// Create invkfile.cue
	invkfileContent := `
cmds: [
	{
		name: "hello"
		description: "Say hello"
		implementations: [
			{
				script: "echo hello"
				runtimes: [{name: "virtual"}]
			},
		]
	},
]
`
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte(invkfileContent), 0o644); err != nil {
		b.Fatalf("Failed to write invkfile: %v", err)
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

		if len(files) == 0 || files[0].Invkfile == nil {
			b.Fatal("No invkfile found")
		}

		inv := files[0].Invkfile
		cmd := inv.GetCommand("hello")
		if cmd == nil {
			b.Fatal("Command 'hello' not found")
		}

		// Execution phase
		ctx := runtime.NewExecutionContext(cmd, inv)
		ctx.Context = context.Background()
		ctx.SelectedRuntime = invkfile.RuntimeVirtual
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
	// Parse the complex invkfile once
	data := []byte(complexInvkfile)
	inv, err := invkfile.ParseBytes(data, "complex.cue")
	if err != nil {
		b.Fatalf("ParseBytes failed: %v", err)
	}

	commandNames := []string{"cmd1", "cmd5", "cmd8", "nested cmd1", "nested cmd2"}

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
	invkfilePath := filepath.Join(tmpDir, "invkfile.cue")

	inv := &invkfile.Invkfile{
		FilePath: invkfilePath,
		Env: &invkfile.EnvConfig{
			Vars: map[string]string{
				"ROOT_VAR1": "value1",
				"ROOT_VAR2": "value2",
			},
		},
		Commands: []invkfile.Command{
			{
				Name:        "test",
				Description: "Test command",
				Env: &invkfile.EnvConfig{
					Vars: map[string]string{
						"CMD_VAR1": "cmd_value1",
						"CMD_VAR2": "cmd_value2",
					},
				},
				Implementations: []invkfile.Implementation{
					{
						Script: "echo test",
						Runtimes: []invkfile.RuntimeConfig{
							{Name: invkfile.RuntimeNative},
						},
						Env: &invkfile.EnvConfig{
							Vars: map[string]string{
								"IMPL_VAR1": "impl_value1",
							},
						},
					},
				},
			},
		},
	}

	cmd := inv.GetCommand("test")
	ctx := runtime.NewExecutionContext(cmd, inv)
	ctx.Context = context.Background()

	envBuilder := runtime.NewDefaultEnvBuilder()

	b.ResetTimer()
	for b.Loop() {
		_, err := envBuilder.Build(ctx, invkfile.EnvInheritAll)
		if err != nil {
			b.Fatalf("Build failed: %v", err)
		}
	}
}

// BenchmarkModuleValidation benchmarks module validation.
func BenchmarkModuleValidation(b *testing.B) {
	// Create a complete module structure
	// Note: The module name in invkmod.cue must match the folder name (minus .invkmod suffix)
	tmpDir := b.TempDir()
	modDir := filepath.Join(tmpDir, "benchmark.invkmod")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		b.Fatalf("Failed to create module dir: %v", err)
	}

	// Create invkmod.cue with module name matching folder
	invkmodContent := `
module: "benchmark"
version: "1.0"
description: "Benchmark test module for PGO profiling"
`
	if err := os.WriteFile(filepath.Join(modDir, "invkmod.cue"), []byte(invkmodContent), 0o644); err != nil {
		b.Fatalf("Failed to write invkmod.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "invkfile.cue"), []byte(sampleInvkfile), 0o644); err != nil {
		b.Fatalf("Failed to write invkfile.cue: %v", err)
	}

	b.ResetTimer()
	for b.Loop() {
		result, err := invkmod.Validate(modDir)
		if err != nil {
			b.Fatalf("Module validation error: %v", err)
		}
		if !result.Valid {
			b.Fatalf("Module validation failed: %v", result.Issues)
		}
	}
}
