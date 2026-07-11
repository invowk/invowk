// SPDX-License-Identifier: MPL-2.0

package discovery

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/invowkmod"
)

// createVendoredModule creates a vendored module inside a parent module's invowk_modules/ dir.
func TestDiscoverModules_SkipsVendoredModuleWithAmbiguousDeclaredLockEntries(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	parentDir := filepath.Join(tmpDir, "io.example.parent.invowkmod")
	createTestModule(t, parentDir, "io.example.parent", "parent-cmd")
	vendorDir := filepath.Join(parentDir, invowkmod.VendoredModulesDir)
	vendoredDir := filepath.Join(vendorDir, "io.example.shared.invowkmod")
	createTestModule(t, vendoredDir, "io.example.shared", "run")

	parentMod := `module: "io.example.parent"
version: "1.0.0"
requires: [
	{git_url: "https://example.com/shared-a.git", version: "^1.0.0"},
	{git_url: "https://example.com/shared-b.git", version: "^1.0.0"},
]
`
	if err := os.WriteFile(filepath.Join(parentDir, "invowkmod.cue"), []byte(parentMod), 0o644); err != nil {
		t.Fatalf("failed to write parent invowkmod.cue: %v", err)
	}

	hash, err := invowkmod.ComputeModuleHash(vendoredDir)
	if err != nil {
		t.Fatalf("ComputeModuleHash() = %v", err)
	}
	lock := invowkmod.NewLockFile()
	for _, key := range []invowkmod.ModuleRefKey{
		"https://example.com/shared-a.git",
		"https://example.com/shared-b.git",
	} {
		lock.Modules[key] = invowkmod.LockedModule{
			GitURL:          invowkmod.GitURL(key),
			Version:         "^1.0.0",
			ResolvedVersion: "1.0.0",
			GitCommit:       "0123456789abcdef0123456789abcdef01234567",
			Namespace:       "io.example.shared",
			CommandSourceID: "io.example.shared",
			ModuleID:        "io.example.shared",
			ContentHash:     hash,
		}
	}
	if saveErr := lock.Save(filepath.Join(parentDir, invowkmod.LockFileName)); saveErr != nil {
		t.Fatalf("lock.Save() = %v", saveErr)
	}

	d := newTestDiscovery(t, config.DefaultConfig(), tmpDir)
	result, err := d.DiscoverModules()
	if err != nil {
		t.Fatalf("DiscoverModules() = %v", err)
	}

	assertAmbiguousVendoredModuleSkipped(t, result)
}

func assertAmbiguousVendoredModuleSkipped(t *testing.T, result ModuleListResult) {
	t.Helper()

	assertModuleIDAbsent(t, result.Modules, "io.example.shared")
	assertAmbiguousVendoredDiagnostic(t, result.Diagnostics)
}

func assertModuleIDAbsent(t *testing.T, modules []*DiscoveredFile, moduleID invowkmod.ModuleID) {
	t.Helper()

	for _, module := range modules {
		if module.Module != nil && module.Module.Metadata.Module == moduleID {
			t.Fatal("ambiguous vendored module was discovered")
		}
	}
}

func assertAmbiguousVendoredDiagnostic(t *testing.T, diagnostics []Diagnostic) {
	t.Helper()

	for _, diag := range diagnostics {
		if diag.code != CodeVendoredAmbiguousLockSkipped {
			continue
		}
		if !strings.Contains(diag.message, "shared-a.git") || !strings.Contains(diag.message, "shared-b.git") {
			t.Fatalf("ambiguous diagnostic message = %q, want both lock keys", diag.message)
		}
		return
	}
	t.Fatalf("diagnostics missing %s: %#v", CodeVendoredAmbiguousLockSkipped, diagnostics)
}

func TestDiscoverAll_NestedVendoredNotRecursed(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	parentDir := filepath.Join(tmpDir, "parent.invowkmod")
	createTestModule(t, parentDir, "parent", "parent-cmd")

	// Create vendored module with its OWN invowk_modules/
	vendoredDir := createVendoredModule(t, parentDir, "mid.invowkmod", "mid", "mid-cmd")
	nestedVendorDir := filepath.Join(vendoredDir, invowkmod.VendoredModulesDir)
	if err := os.MkdirAll(nestedVendorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	createTestModule(t, filepath.Join(nestedVendorDir, "deep.invowkmod"), "deep", "deep-cmd")
	refreshVendoredModuleHash(t, parentDir, vendoredDir, "mid")

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	files, _, err := d.discoverAllWithDiagnostics()
	if err != nil {
		t.Fatalf("discoverAllWithDiagnostics() error: %v", err)
	}

	// "mid" should be found, "deep" should NOT
	var foundMid, foundDeep bool
	for _, f := range files {
		if f.Module != nil && f.Module.Name() == "mid" {
			foundMid = true
		}
		if f.Module != nil && f.Module.Name() == "deep" {
			foundDeep = true
		}
	}

	if !foundMid {
		t.Error("first-level vendored module 'mid' should be discovered")
	}
	if foundDeep {
		t.Error("nested vendored module 'deep' should NOT be discovered (no recursion)")
	}
}

func TestDiscoverAll_NestedVendoredEmitsDiagnostic(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	parentDir := filepath.Join(tmpDir, "parent.invowkmod")
	createTestModule(t, parentDir, "parent", "parent-cmd")

	vendoredDir := createVendoredModule(t, parentDir, "mid.invowkmod", "mid", "mid-cmd")

	// Create invowk_modules/ inside the vendored module (triggering the warning)
	nestedVendorDir := filepath.Join(vendoredDir, invowkmod.VendoredModulesDir)
	if err := os.MkdirAll(nestedVendorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	refreshVendoredModuleHash(t, parentDir, vendoredDir, "mid")

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	_, diagnostics, err := d.discoverAllWithDiagnostics()
	if err != nil {
		t.Fatalf("discoverAllWithDiagnostics() error: %v", err)
	}

	var foundNestedWarning bool
	for _, diag := range diagnostics {
		if diag.code == "vendored_nested_ignored" {
			foundNestedWarning = true
			break
		}
	}
	if !foundNestedWarning {
		t.Error("should emit 'vendored_nested_ignored' diagnostic when vendored module has its own invowk_modules/")
	}
}

func TestDiscoverAll_InvalidVendoredModuleSkipped(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	parentDir := filepath.Join(tmpDir, "parent.invowkmod")
	createTestModule(t, parentDir, "parent", "parent-cmd")

	// Create an invalid vendored module (dir with .invowkmod suffix but no invowkmod.cue)
	vendorDir := filepath.Join(parentDir, invowkmod.VendoredModulesDir)
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	invalidDir := filepath.Join(vendorDir, "broken.invowkmod")
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	_, diagnostics, err := d.discoverAllWithDiagnostics()
	if err != nil {
		t.Fatalf("discoverAllWithDiagnostics() error: %v", err)
	}

	var foundSkipDiag bool
	for _, diag := range diagnostics {
		if diag.code == "vendored_module_load_skipped" {
			foundSkipDiag = true
			break
		}
	}
	if !foundSkipDiag {
		t.Error("should emit 'vendored_module_load_skipped' diagnostic for invalid vendored module")
	}
}

func TestDiscoverAll_EmptyVendorDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	parentDir := filepath.Join(tmpDir, "parent.invowkmod")
	createTestModule(t, parentDir, "parent", "parent-cmd")

	// Create empty invowk_modules/
	vendorDir := filepath.Join(parentDir, invowkmod.VendoredModulesDir)
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	files, diagnostics, err := d.discoverAllWithDiagnostics()
	if err != nil {
		t.Fatalf("discoverAllWithDiagnostics() error: %v", err)
	}

	// Only parent should be found, no vendored
	for _, f := range files {
		if f.ParentModule != nil {
			t.Error("should not find vendored modules in empty vendor dir")
		}
	}

	// No vendor-related diagnostics
	for _, diag := range diagnostics {
		if strings.HasPrefix(string(diag.code), "vendored_") {
			t.Errorf("unexpected vendor diagnostic: %s", diag.code)
		}
	}
}

func TestDiscoverAll_VendoredReservedModuleSkipped(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a parent module
	parentDir := filepath.Join(tmpDir, "parent.invowkmod")
	createTestModule(t, parentDir, "parent", "parent-cmd")

	// Create a vendored directory with the reserved "invowkfile" name.
	// The reserved name check fires before Load(), so no invowkmod.cue is needed inside.
	vendorDir := filepath.Join(parentDir, invowkmod.VendoredModulesDir)
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatal(err)
	}
	reservedDir := filepath.Join(vendorDir, "invowkfile.invowkmod")
	if err := os.MkdirAll(reservedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	files, diagnostics, err := d.discoverAllWithDiagnostics()
	if err != nil {
		t.Fatalf("discoverAllWithDiagnostics() error: %v", err)
	}

	// Verify no DiscoveredFile has module name "invowkfile"
	for _, f := range files {
		if f.Module != nil && f.Module.Name() == "invowkfile" {
			t.Error("should not discover a module with reserved name 'invowkfile'")
		}
	}

	// Verify diagnostic was emitted
	var foundDiag bool
	for _, diag := range diagnostics {
		if diag.code == "vendored_reserved_module_skipped" {
			foundDiag = true
			break
		}
	}
	if !foundDiag {
		t.Error("should emit 'vendored_reserved_module_skipped' diagnostic for reserved module name in vendor dir")
	}
}

func TestDiscoverAll_VendoredScanFailed(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("skipping: Windows does not use POSIX file permissions")
	}
	if os.Getuid() == 0 {
		t.Skip("skipping: root can read any directory regardless of permissions")
	}

	tmpDir := t.TempDir()

	parentDir := filepath.Join(tmpDir, "parent.invowkmod")
	createTestModule(t, parentDir, "parent", "parent-cmd")

	// Create invowk_modules/ and make it unreadable
	vendorDir := filepath.Join(parentDir, invowkmod.VendoredModulesDir)
	if err := os.MkdirAll(vendorDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(vendorDir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		// Restore permissions so t.TempDir() cleanup can remove it
		_ = os.Chmod(vendorDir, 0o750)
	})

	cfg := config.DefaultConfig()
	d := newTestDiscovery(t, cfg, tmpDir)

	// discoverAllWithDiagnostics should NOT return an error (non-fatal)
	_, diagnostics, err := d.discoverAllWithDiagnostics()
	if err != nil {
		t.Fatalf("discoverAllWithDiagnostics() should not return error for unreadable vendor dir: %v", err)
	}

	var foundDiag bool
	for _, diag := range diagnostics {
		if diag.code == "vendored_scan_failed" {
			foundDiag = true
			break
		}
	}
	if !foundDiag {
		t.Error("should emit 'vendored_scan_failed' diagnostic when vendor directory is unreadable")
	}
}
