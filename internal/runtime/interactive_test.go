// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/invowkfile"
)

// TestInteractiveRuntimeInterface verifies that all runtimes implement InteractiveRuntime.
func TestInteractiveRuntimeInterface(t *testing.T) {
	t.Parallel()

	t.Run("NativeRuntime implements InteractiveRuntime", func(t *testing.T) {
		t.Parallel()

		var rt Runtime = NewNativeRuntime()
		ir, ok := rt.(InteractiveRuntime)
		if !ok {
			t.Error("NativeRuntime does not implement InteractiveRuntime")
		}
		if !ir.SupportsInteractive() {
			t.Error("NativeRuntime.SupportsInteractive() returned false, expected true")
		}
	})

	t.Run("VirtualRuntime implements InteractiveRuntime", func(t *testing.T) {
		t.Parallel()

		var rt Runtime = NewVirtualRuntime(false)
		ir, ok := rt.(InteractiveRuntime)
		if !ok {
			t.Error("VirtualRuntime does not implement InteractiveRuntime")
		}
		if !ir.SupportsInteractive() {
			t.Error("VirtualRuntime.SupportsInteractive() returned false, expected true")
		}
	})

	t.Run("ContainerRuntime implements InteractiveRuntime", func(t *testing.T) {
		t.Parallel()

		cfg := &config.Config{ContainerEngine: "docker"}
		crt, err := NewContainerRuntime(cfg)
		if err != nil {
			t.Skipf("Container runtime not available: %v", err)
		}

		var rt Runtime = crt
		ir, ok := rt.(InteractiveRuntime)
		if !ok {
			t.Error("ContainerRuntime does not implement InteractiveRuntime")
		}
		// SupportsInteractive depends on engine availability
		_ = ir.SupportsInteractive()
	})
}

// TestGetInteractiveRuntime tests the helper function for getting InteractiveRuntime.
func TestGetInteractiveRuntime(t *testing.T) {
	t.Parallel()

	t.Run("returns InteractiveRuntime for NativeRuntime", func(t *testing.T) {
		t.Parallel()

		rt := NewNativeRuntime()
		ir := GetInteractiveRuntime(rt)
		if ir == nil {
			t.Error("GetInteractiveRuntime returned nil for NativeRuntime")
		}
	})

	t.Run("returns InteractiveRuntime for VirtualRuntime", func(t *testing.T) {
		t.Parallel()

		rt := NewVirtualRuntime(false)
		ir := GetInteractiveRuntime(rt)
		if ir == nil {
			t.Error("GetInteractiveRuntime returned nil for VirtualRuntime")
		}
	})
}

// TestNativeRuntimePrepareInteractive tests the NativeRuntime.PrepareInteractive method.
func TestNativeRuntimePrepareInteractive(t *testing.T) {
	t.Parallel()

	// Create a temporary invowkfile (module metadata now in invowkmod.cue, not invowkfile.cue)
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	err := os.WriteFile(invowkfilePath, []byte(`
cmds: [{
	name: "hello"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
	}]
}]
`), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test invowkfile: %v", err)
	}

	// Parse the invowkfile
	inv, err := invowkfile.Parse(invowkfile.FilesystemPath(invowkfilePath))
	if err != nil {
		t.Fatalf("Failed to parse invowkfile: %v", err)
	}

	// Create execution context
	ctx := NewExecutionContext(t.Context(), &inv.Commands[0], inv)

	// Create native runtime and prepare for interactive execution
	rt := NewNativeRuntime()
	prepared, err := rt.PrepareInteractive(ctx)
	if err != nil {
		t.Fatalf("PrepareInteractive failed: %v", err)
	}

	// Verify prepared command
	if prepared.Cmd == nil {
		t.Fatal("PrepareInteractive returned nil Cmd")
	}

	// Cleanup
	if prepared.Cleanup != nil {
		prepared.Cleanup()
	}
}

// TestVirtualRuntimePrepareInteractive tests the VirtualRuntime.PrepareInteractive method.
func TestVirtualRuntimePrepareInteractive(t *testing.T) {
	t.Parallel()

	// Create a temporary invowkfile (module metadata now in invowkmod.cue, not invowkfile.cue)
	tmpDir := t.TempDir()
	invowkfilePath := filepath.Join(tmpDir, "invowkfile.cue")

	err := os.WriteFile(invowkfilePath, []byte(`
cmds: [{
	name: "hello"
	implementations: [{
		script: "echo hello"
		runtimes: [{name: "virtual"}]
		platforms: [{name: "linux"}, {name: "macos"}, {name: "windows"}]
	}]
}]
`), 0o644)
	if err != nil {
		t.Fatalf("Failed to create test invowkfile: %v", err)
	}

	// Parse the invowkfile
	inv, err := invowkfile.Parse(invowkfile.FilesystemPath(invowkfilePath))
	if err != nil {
		t.Fatalf("Failed to parse invowkfile: %v", err)
	}

	// Create execution context
	ctx := NewExecutionContext(t.Context(), &inv.Commands[0], inv)

	ctx.SelectedRuntime = invowkfile.RuntimeVirtual
	ctx.SelectedImpl = &inv.Commands[0].Implementations[0]

	var gotSpec VirtualInteractiveCommandSpec
	factory := func(ctx context.Context, spec VirtualInteractiveCommandSpec) (*exec.Cmd, error) {
		gotSpec = spec
		return exec.CommandContext(ctx, "test-invowk", "virtual-launcher"), nil
	}

	// Create virtual runtime and prepare for interactive execution
	rt := NewVirtualRuntime(false, WithInteractiveCommandFactory(factory))
	prepared, err := rt.PrepareInteractive(ctx)
	if err != nil {
		t.Fatalf("PrepareInteractive failed: %v", err)
	}

	// Verify prepared command
	if prepared.Cmd == nil {
		t.Fatal("PrepareInteractive returned nil Cmd")
	}

	if prepared.Cmd.Args[0] != "test-invowk" || prepared.Cmd.Args[1] != "virtual-launcher" {
		t.Fatalf("prepared command args = %v, want injected launcher", prepared.Cmd.Args)
	}
	if gotSpec.ScriptFile == nil {
		t.Fatal("launcher spec missing script file")
	}
	data, err := os.ReadFile(string(*gotSpec.ScriptFile))
	if err != nil {
		t.Fatalf("ReadFile(script file) error = %v", err)
	}
	if string(data) != "echo hello" {
		t.Fatalf("script file contents = %q, want echo hello", data)
	}
	if gotSpec.WorkDir == nil {
		t.Fatal("launcher spec missing workdir")
	}
	if gotSpec.EnvJSON == "" {
		t.Fatal("launcher spec missing env JSON")
	}
	if gotSpec.EnableUroot {
		t.Fatal("launcher spec EnableUroot = true, want false")
	}

	// Cleanup (removes temp script file)
	if prepared.Cleanup != nil {
		prepared.Cleanup()
	}
}

func TestVirtualRuntimePrepareInteractivePassesUrootPolicy(t *testing.T) {
	t.Parallel()

	inv := &invowkfile.Invowkfile{
		Commands: []invowkfile.Command{{
			Name: "list",
			Implementations: []invowkfile.Implementation{{
				Script:    "ls",
				Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeVirtual}},
				Platforms: invowkfile.AllPlatformConfigs(),
			}},
		}},
	}
	ctx := NewExecutionContext(t.Context(), &inv.Commands[0], inv)
	ctx.SelectedRuntime = invowkfile.RuntimeVirtual
	ctx.SelectedImpl = &inv.Commands[0].Implementations[0]

	var gotSpec VirtualInteractiveCommandSpec
	factory := func(ctx context.Context, spec VirtualInteractiveCommandSpec) (*exec.Cmd, error) {
		gotSpec = spec
		return exec.CommandContext(ctx, "test-invowk", "virtual-launcher"), nil
	}

	rt := NewVirtualRuntime(true, WithInteractiveCommandFactory(factory))
	prepared, err := rt.PrepareInteractive(ctx)
	if err != nil {
		t.Fatalf("PrepareInteractive() error = %v", err)
	}
	t.Cleanup(prepared.Cleanup)

	if !gotSpec.EnableUroot {
		t.Fatal("launcher spec EnableUroot = false, want true")
	}
}

// TestContainerRuntimeGetHostAddressForContainer tests the host address resolution.
func TestContainerRuntimeGetHostAddressForContainer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want HostServiceAddress
	}{
		{"docker", hostDockerInternal},
		{"podman", hostContainersInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rt, err := NewContainerRuntimeWithEngine(NewMockEngine().WithName(tt.name))
			if err != nil {
				t.Fatalf("NewContainerRuntimeWithEngine() error = %v", err)
			}
			var provider HostServiceAddressProvider = rt
			if got := provider.HostServiceAddress(); got != tt.want {
				t.Fatalf("HostServiceAddress() = %q, want %q", got, tt.want)
			}
			if got := rt.GetHostAddressForContainer(); got != tt.want.String() {
				t.Fatalf("GetHostAddressForContainer() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestPreparedCommandCleanup verifies that cleanup functions work correctly.
func TestPreparedCommandCleanup(t *testing.T) {
	t.Parallel()

	// Create a temp file to simulate cleanup
	tmpFile, err := os.CreateTemp("", "test-cleanup-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	testutil.MustClose(t, tmpFile)

	// Create a prepared command with cleanup
	prepared := &PreparedCommand{
		Cleanup: func() {
			_ = os.Remove(tmpPath) // Cleanup temp file; error non-critical
		},
	}

	// Verify file exists before cleanup
	if _, err := os.Stat(tmpPath); os.IsNotExist(err) {
		t.Fatal("Temp file should exist before cleanup")
	}

	// Call cleanup
	prepared.Cleanup()

	// Verify file is removed after cleanup
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("Temp file should be removed after cleanup")
	}
}
