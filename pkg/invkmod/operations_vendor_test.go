// SPDX-License-Identifier: MPL-2.0

package invkmod

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ============================================================================
// Tests for Vendored Modules
// ============================================================================

func TestVendoredModulesDir(t *testing.T) {
	if VendoredModulesDir != "invk_modules" {
		t.Errorf("VendoredModulesDir = %q, want %q", VendoredModulesDir, "invk_modules")
	}
}

func TestGetVendoredModulesDir(t *testing.T) {
	modulePath := "/path/to/mymodule.invkmod"
	expected := filepath.Join(modulePath, "invk_modules")
	result := GetVendoredModulesDir(modulePath)
	if result != expected {
		t.Errorf("GetVendoredModulesDir() = %q, want %q", result, expected)
	}
}

func TestHasVendoredModules(t *testing.T) {
	t.Run("no vendored modules directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		modulePath := createValidModuleForPackaging(t, tmpDir, "mymodule.invkmod", "mymodule")

		if HasVendoredModules(modulePath) {
			t.Error("HasVendoredModules() should return false when invk_modules/ doesn't exist")
		}
	})

	t.Run("empty vendored modules directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		modulePath := createValidModuleForPackaging(t, tmpDir, "mymodule.invkmod", "mymodule")
		vendoredDir := filepath.Join(modulePath, VendoredModulesDir)
		if err := os.Mkdir(vendoredDir, 0o755); err != nil {
			t.Fatal(err)
		}

		if HasVendoredModules(modulePath) {
			t.Error("HasVendoredModules() should return false when invk_modules/ is empty")
		}
	})

	t.Run("with vendored modules", func(t *testing.T) {
		tmpDir := t.TempDir()
		modulePath := createValidModuleForPackaging(t, tmpDir, "mymodule.invkmod", "mymodule")
		vendoredDir := filepath.Join(modulePath, VendoredModulesDir)
		if err := os.Mkdir(vendoredDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// Create a vendored module using new format
		createValidModuleForPackaging(t, vendoredDir, "vendor.invkmod", "vendor")

		if !HasVendoredModules(modulePath) {
			t.Error("HasVendoredModules() should return true when invk_modules/ has modules")
		}
	})
}

func TestListVendoredModules(t *testing.T) {
	t.Run("no vendored modules", func(t *testing.T) {
		tmpDir := t.TempDir()
		modulePath := filepath.Join(tmpDir, "mymodule.invkmod")
		if err := os.Mkdir(modulePath, 0o755); err != nil {
			t.Fatal(err)
		}

		modules, err := ListVendoredModules(modulePath)
		if err != nil {
			t.Fatalf("ListVendoredModules() error: %v", err)
		}
		if len(modules) != 0 {
			t.Errorf("ListVendoredModules() returned %d modules, want 0", len(modules))
		}
	})

	t.Run("with vendored modules", func(t *testing.T) {
		tmpDir := t.TempDir()
		modulePath := filepath.Join(tmpDir, "mymodule.invkmod")
		if err := os.Mkdir(modulePath, 0o755); err != nil {
			t.Fatal(err)
		}
		vendoredDir := filepath.Join(modulePath, VendoredModulesDir)
		if err := os.Mkdir(vendoredDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Create two vendored modules using new format
		createValidModuleForPackaging(t, vendoredDir, "vendor1.invkmod", "vendor1")
		createValidModuleForPackaging(t, vendoredDir, "vendor2.invkmod", "vendor2")

		modules, err := ListVendoredModules(modulePath)
		if err != nil {
			t.Fatalf("ListVendoredModules() error: %v", err)
		}
		if len(modules) != 2 {
			t.Errorf("ListVendoredModules() returned %d modules, want 2", len(modules))
		}

		// Check module names
		names := make(map[string]bool)
		for _, p := range modules {
			names[p.Name()] = true
		}
		if !names["vendor1"] || !names["vendor2"] {
			t.Errorf("ListVendoredModules() missing expected modules, got: %v", names)
		}
	})

	t.Run("skips invalid modules", func(t *testing.T) {
		tmpDir := t.TempDir()
		modulePath := filepath.Join(tmpDir, "mymodule.invkmod")
		if err := os.Mkdir(modulePath, 0o755); err != nil {
			t.Fatal(err)
		}
		vendoredDir := filepath.Join(modulePath, VendoredModulesDir)
		if err := os.Mkdir(vendoredDir, 0o755); err != nil {
			t.Fatal(err)
		}

		// Create a valid module using new format
		createValidModuleForPackaging(t, vendoredDir, "valid.invkmod", "valid")

		// Create an invalid module (no invkmod.cue)
		invalidModule := filepath.Join(vendoredDir, "invalid.invkmod")
		if err := os.Mkdir(invalidModule, 0o755); err != nil {
			t.Fatal(err)
		}

		modules, err := ListVendoredModules(modulePath)
		if err != nil {
			t.Fatalf("ListVendoredModules() error: %v", err)
		}
		if len(modules) != 1 {
			t.Errorf("ListVendoredModules() returned %d modules, want 1 (should skip invalid)", len(modules))
		}
		if len(modules) > 0 && modules[0].Name() != "valid" {
			t.Errorf("ListVendoredModules() returned wrong module: %s", modules[0].Name())
		}
	})
}

// ============================================================================
// Tests for Validate with Nested/Vendored Modules
// ============================================================================

func TestValidate_AllowsNestedModulesInVendoredDir(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "mycommands.invkmod", "mycommands")

	// Create invk_modules directory with a nested module
	vendoredDir := filepath.Join(modulePath, VendoredModulesDir)
	if err := os.Mkdir(vendoredDir, 0o755); err != nil {
		t.Fatal(err)
	}
	createValidModuleForPackaging(t, vendoredDir, "vendored.invkmod", "vendored")

	result, err := Validate(modulePath)
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	if !result.Valid {
		t.Errorf("Validate() should return valid for module with nested modules in invk_modules/. Issues: %v", result.Issues)
	}
}

func TestValidate_StillRejectsNestedModulesOutsideVendoredDir(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "mycommands.invkmod", "mycommands")

	// Create a nested module NOT in invk_modules
	nestedModule := filepath.Join(modulePath, "nested.invkmod")
	if err := os.Mkdir(nestedModule, 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := Validate(modulePath)
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	if result.Valid {
		t.Error("Validate() should reject nested modules outside of invk_modules/")
	}

	// Check that the issue mentions nested module
	foundNestedIssue := false
	for _, issue := range result.Issues {
		if strings.Contains(issue.Message, "nested") {
			foundNestedIssue = true
			break
		}
	}
	if !foundNestedIssue {
		t.Error("Validate() should report issue about nested module")
	}
}

func TestValidate_DetectsSymlinks(t *testing.T) {
	// Skip on Windows since symlinks work differently
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows")
	}

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "mycommands.invkmod", "mycommands")

	// Create a file outside the module
	outsideFile := filepath.Join(tmpDir, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("outside content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside the module pointing outside
	symlinkPath := filepath.Join(modulePath, "link_to_outside")
	if err := os.Symlink(outsideFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	result, err := Validate(modulePath)
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	// Should report a security issue about the symlink
	foundSymlinkIssue := false
	for _, issue := range result.Issues {
		if issue.Type == "security" && strings.Contains(strings.ToLower(issue.Message), "symlink") {
			foundSymlinkIssue = true
			break
		}
	}
	if !foundSymlinkIssue {
		t.Error("Validate() should report security issue about symlink pointing outside module")
	}
}

func TestValidate_DetectsWindowsReservedFilenames(t *testing.T) {
	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "mycommands.invkmod", "mycommands")

	// Create a file with a Windows reserved name
	reservedFile := filepath.Join(modulePath, "CON")
	if err := os.WriteFile(reservedFile, []byte("test"), 0o644); err != nil {
		// On Windows, this might fail - that's expected
		if runtime.GOOS == "windows" {
			t.Skip("Cannot create reserved filename on Windows")
		}
		t.Fatal(err)
	}

	result, err := Validate(modulePath)
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	// Should report a compatibility issue about the reserved filename
	foundReservedIssue := false
	for _, issue := range result.Issues {
		if issue.Type == "compatibility" && strings.Contains(issue.Message, "reserved on Windows") {
			foundReservedIssue = true
			break
		}
	}
	if !foundReservedIssue {
		t.Error("Validate() should report compatibility issue about Windows reserved filename")
	}
}

func TestValidate_RejectsAllSymlinks(t *testing.T) {
	// Skip on Windows since symlinks work differently
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows")
	}

	tmpDir := t.TempDir()
	modulePath := createValidModuleForPackaging(t, tmpDir, "mycommands.invkmod", "mycommands")

	// Create scripts directory
	scriptsDir := filepath.Join(modulePath, "scripts")
	if err := os.Mkdir(scriptsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a file inside the module
	internalFile := filepath.Join(scriptsDir, "original.sh")
	if err := os.WriteFile(internalFile, []byte("#!/bin/bash\necho hello"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside the module pointing to another file inside the module
	symlinkPath := filepath.Join(modulePath, "link_to_internal")
	if err := os.Symlink(internalFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	result, err := Validate(modulePath)
	if err != nil {
		t.Fatalf("Validate() returned error: %v", err)
	}

	// ALL symlinks should be rejected as a security measure (even internal ones)
	// This is intentional to prevent zip slip attacks during archive extraction
	foundSecurityIssue := false
	for _, issue := range result.Issues {
		if issue.Type == "security" && strings.Contains(strings.ToLower(issue.Message), "symlink") {
			foundSecurityIssue = true
			break
		}
	}
	if !foundSecurityIssue {
		t.Error("Validate() should report security issue for ALL symlinks (including internal ones)")
	}
}
