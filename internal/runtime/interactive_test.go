// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"context"
	"os"
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
	inv, err := invowkfile.Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Failed to parse invowkfile: %v", err)
	}

	// Create execution context
	ctx := NewExecutionContext(&inv.Commands[0], inv)
	ctx.Context = context.Background()

	// Create native runtime and prepare for interactive execution
	rt := NewNativeRuntime()
	prepared, err := rt.PrepareInteractive(ctx)
	if err != nil {
		t.Fatalf("PrepareInteractive failed: %v", err)
	}

	// Verify prepared command
	if prepared.Cmd == nil {
		t.Error("PrepareInteractive returned nil Cmd")
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
	inv, err := invowkfile.Parse(invowkfilePath)
	if err != nil {
		t.Fatalf("Failed to parse invowkfile: %v", err)
	}

	// Create execution context
	ctx := NewExecutionContext(&inv.Commands[0], inv)
	ctx.Context = context.Background()
	ctx.SelectedRuntime = invowkfile.RuntimeVirtual
	ctx.SelectedImpl = &inv.Commands[0].Implementations[0]

	// Create virtual runtime and prepare for interactive execution
	rt := NewVirtualRuntime(false)
	prepared, err := rt.PrepareInteractive(ctx)
	if err != nil {
		t.Fatalf("PrepareInteractive failed: %v", err)
	}

	// Verify prepared command
	if prepared.Cmd == nil {
		t.Error("PrepareInteractive returned nil Cmd")
	}

	// Verify the command invokes invowk internal exec-virtual
	args := prepared.Cmd.Args
	if len(args) < 3 {
		t.Errorf("Expected at least 3 args, got %d", len(args))
	} else if args[1] != "internal" || args[2] != "exec-virtual" {
		t.Errorf("Expected 'internal exec-virtual' args, got %v", args[1:3])
	}

	// Cleanup (removes temp script file)
	if prepared.Cleanup != nil {
		prepared.Cleanup()
	}
}

// TestContainerRuntimeGetHostAddressForContainer tests the host address resolution.
func TestContainerRuntimeGetHostAddressForContainer(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{ContainerEngine: "docker"}
	rt, err := NewContainerRuntime(cfg)
	if err != nil {
		t.Skipf("Container runtime not available: %v", err)
	}

	hostAddr := rt.GetHostAddressForContainer()
	if hostAddr != "host.docker.internal" && hostAddr != "host.containers.internal" {
		t.Errorf("Unexpected host address: %s", hostAddr)
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
