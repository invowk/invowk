// SPDX-License-Identifier: MPL-2.0

package runtime

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/internal/container"
	"github.com/invowk/invowk/internal/provision"
	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/invowkfile"
	"github.com/invowk/invowk/pkg/types"
)

// These tests use only fixtures whose verdict does not depend on the host
// (Mach-O, PE, a missing file): the unit tests also run on macOS and Windows,
// and the host-dependent branches are covered in internal/provision.

func TestBuildProvisionConfig_BinaryPathOnlyFromConfig(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Container.AutoProvision.BinaryPath = "/opt/invowk-linux-amd64/invowk"

	provCfg := buildProvisionConfig(cfg)

	if provCfg.InvowkBinaryPath != "/opt/invowk-linux-amd64/invowk" {
		t.Fatalf("InvowkBinaryPath = %q, want the configured binary_path", provCfg.InvowkBinaryPath)
	}
	if provCfg.DefaultInvowkBinaryPath == "" {
		t.Fatal("DefaultInvowkBinaryPath is empty, want the running executable kept apart")
	}
	if provCfg.DefaultInvowkBinaryPath == provCfg.InvowkBinaryPath {
		t.Fatalf("DefaultInvowkBinaryPath = InvowkBinaryPath = %q, want them apart", provCfg.InvowkBinaryPath)
	}
}

// newBinaryProvisionRuntime returns a runtime with a real LayerProvisioner
// over engine, with the given binary paths.
func newBinaryProvisionRuntime(t *testing.T, engine *MockEngine, strict bool, configured, defaultPath string) *ContainerRuntime {
	t.Helper()
	rt, err := NewContainerRuntimeWithEngine(engine)
	if err != nil {
		t.Fatalf("NewContainerRuntimeWithEngine() error = %v", err)
	}
	provCfg := &provision.Config{
		Enabled:                 true,
		Strict:                  strict,
		InvowkBinaryPath:        types.FilesystemPath(configured),
		DefaultInvowkBinaryPath: types.FilesystemPath(defaultPath),
		CacheDir:                types.FilesystemPath(t.TempDir()),
		BinaryMountPath:         container.MountTargetPath("/invowk/bin"),
		ModulesMountPath:        container.MountTargetPath("/invowk/modules"),
	}
	if err := rt.SetProvisionConfig(provCfg); err != nil {
		t.Fatalf("SetProvisionConfig() error = %v", err)
	}
	return rt
}

func newBinaryProvisionContext(t *testing.T) *ExecutionContext {
	t.Helper()
	tmpDir := t.TempDir()
	inv := &invowkfile.Invowkfile{FilePath: invowkfile.FilesystemPath(filepath.Join(tmpDir, "invowkfile.cue"))}
	cmd := &invowkfile.Command{
		Name: "binary-test",
		Implementations: []invowkfile.Implementation{{
			Script:    invowkfile.ImplementationScript{Content: "echo hello"},
			Runtimes:  []invowkfile.RuntimeConfig{{Name: invowkfile.RuntimeContainer, Image: "debian:stable-slim"}},
			Platforms: invowkfile.AllPlatformConfigs(),
		}},
	}
	execCtx := NewExecutionContext(t.Context(), cmd, inv)
	execCtx.IO.Stdout = &bytes.Buffer{}
	execCtx.IO.Stderr = &bytes.Buffer{}
	return execCtx
}

func TestEnsureProvisionedImage_UnusableExplicitBinaryFailsWithoutFallback(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	tests := []struct {
		name   string
		path   string
		strict bool
	}{
		{name: "Mach-O, strict false", path: testutil.WriteMachOFixture(t, filepath.Join(dir, "macho"))},
		{name: "PE, strict false", path: testutil.WritePEFixture(t, filepath.Join(dir, "pe"))},
		{name: "missing, strict false", path: filepath.Join(dir, "missing")},
		{name: "Mach-O, strict true", path: filepath.Join(dir, "macho"), strict: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			engine := NewMockEngine().WithImageExists(false)
			rt := newBinaryProvisionRuntime(t, engine, tt.strict, tt.path, "")

			image, _, _, diagnostics, err := rt.ensureProvisionedImage(newBinaryProvisionContext(t), invowkfileContainerConfig{Image: "debian:stable-slim"}, dir)

			if !errors.Is(err, provision.ErrUnusableInvowkBinary) {
				t.Fatalf("ensureProvisionedImage() error = %v, want ErrUnusableInvowkBinary", err)
			}
			if errors.Is(err, errStrictModeProvisioning) {
				t.Errorf("error = %v, want the binary error itself, not the strict-mode wrapper", err)
			}
			if !strings.Contains(err.Error(), tt.path) || !strings.Contains(err.Error(), "container.auto_provision.binary_path") {
				t.Errorf("error %q does not name the configured path", err)
			}
			if image != "" || len(diagnostics) != 0 {
				t.Errorf("image = %q, diagnostics = %+v; want no fallback to the base image", image, diagnostics)
			}
			if len(engine.BuildCalls) != 0 {
				t.Errorf("BuildCalls = %d, want none", len(engine.BuildCalls))
			}
		})
	}
}

func TestEnsureProvisionedImage_SkippedDefaultBinaryWarns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	machO := testutil.WriteMachOFixture(t, filepath.Join(dir, "invowk"))

	for _, strict := range []bool{false, true} {
		engine := NewMockEngine().WithImageExists(false)
		rt := newBinaryProvisionRuntime(t, engine, strict, "", machO)

		image, envVars, cleanup, diagnostics, err := rt.ensureProvisionedImage(newBinaryProvisionContext(t), invowkfileContainerConfig{Image: "debian:stable-slim"}, dir)
		if cleanup != nil {
			cleanup()
		}
		if err != nil {
			t.Fatalf("strict=%v: ensureProvisionedImage() error = %v, want the run to continue", strict, err)
		}
		if !strings.HasPrefix(image, "invowk-provisioned:") {
			t.Errorf("strict=%v: image = %q, want a provisioned image", strict, image)
		}
		if _, ok := envVars["PATH"]; ok {
			t.Errorf("strict=%v: env PATH = %q, want none without the binary", strict, envVars["PATH"])
		}
		if len(diagnostics) != 1 || diagnostics[0].Code != CodeContainerProvisioningWarning {
			t.Fatalf("strict=%v: diagnostics = %+v, want one %s", strict, diagnostics, CodeContainerProvisioningWarning)
		}
		if !strings.Contains(diagnostics[0].Message, "container.auto_provision.binary_path") || !strings.Contains(diagnostics[0].Message, "Mach-O") {
			t.Errorf("strict=%v: warning %q, want the reason and binary_path", strict, diagnostics[0].Message)
		}
		if len(engine.BuildCalls) != 1 {
			t.Errorf("strict=%v: BuildCalls = %d, want 1", strict, len(engine.BuildCalls))
		}
	}
}

func TestPersistentSpecImage_UnusableExplicitBinaryFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	rt := newBinaryProvisionRuntime(t, NewMockEngine(), false, testutil.WritePEFixture(t, filepath.Join(dir, "invowk.exe")), "")

	_, err := rt.persistentSpecImage(newBinaryProvisionContext(t), invowkfileContainerConfig{Image: "debian:stable-slim"})
	if !errors.Is(err, provision.ErrUnusableInvowkBinary) {
		t.Fatalf("persistentSpecImage() error = %v, want ErrUnusableInvowkBinary", err)
	}
}

func TestContainerRuntimeExecuteReusedPersistentContainerSkipsBinaryWarning(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	engine := NewMockEngine().WithImageExists(false)
	rt := newBinaryProvisionRuntime(t, engine, false, "", testutil.WriteMachOFixture(t, filepath.Join(dir, "invowk")))
	ctx := newPersistentExecutionContext(t, &invowkfile.RuntimePersistentConfig{CreateIfMissing: true, Name: "managed-dev"})

	first := rt.Execute(ctx)
	if first.Error != nil {
		t.Fatalf("first Execute() error = %v", first.Error)
	}
	if countDiagnostics(first.Diagnostics, CodeContainerProvisioningWarning) != 1 {
		t.Fatalf("first Execute() diagnostics = %+v, want one skip warning", first.Diagnostics)
	}
	if len(engine.CreateCalls) != 1 {
		t.Fatalf("CreateCalls = %d, want 1", len(engine.CreateCalls))
	}

	engine.WithInspectInfo(&container.ContainerInfo{
		ContainerID: "managed-container",
		Name:        "managed-dev",
		Running:     true,
		Labels:      engine.CreateCalls[0].Labels,
	})
	second := rt.Execute(ctx)
	if second.Error != nil {
		t.Fatalf("second Execute() error = %v, want the managed container reused", second.Error)
	}
	if n := countDiagnostics(second.Diagnostics, CodeContainerProvisioningWarning); n != 0 {
		t.Fatalf("second Execute() diagnostics = %+v, want no provisioning warning", second.Diagnostics)
	}
	if len(engine.CreateCalls) != 1 || len(engine.BuildCalls) != 1 {
		t.Fatalf("CreateCalls = %d, BuildCalls = %d, want 1 and 1", len(engine.CreateCalls), len(engine.BuildCalls))
	}
}

func countDiagnostics(diagnostics []InitDiagnostic, code InitDiagnosticCode) int {
	n := 0
	for _, d := range diagnostics {
		if d.Code == code {
			n++
		}
	}
	return n
}
