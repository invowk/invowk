// SPDX-License-Identifier: MPL-2.0

package invkmod

import (
	"archive/zip"
	"fmt"
	"invowk-cli/internal/testutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ============================================================================
// Tests for Module Packaging Operations (Create, Archive, Unpack)
// ============================================================================

// Helper function to create a valid module with both invkmod.cue and invkfile.cue
func createValidModuleForPackaging(t *testing.T, dir, folderName, moduleID string) string {
	t.Helper()
	modulePath := filepath.Join(dir, folderName)
	if err := os.Mkdir(modulePath, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create invkmod.cue with metadata
	invkmodPath := filepath.Join(modulePath, "invkmod.cue")
	invkmodContent := fmt.Sprintf(`module: "%s"
version: "1.0"
`, moduleID)
	if err := os.WriteFile(invkmodPath, []byte(invkmodContent), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create invkfile.cue with commands
	invkfilePath := filepath.Join(modulePath, "invkfile.cue")
	if err := os.WriteFile(invkfilePath, []byte("cmds: []"), 0o644); err != nil {
		t.Fatal(err)
	}
	return modulePath
}

func TestCreate(t *testing.T) {
	tests := []struct {
		name      string
		opts      CreateOptions
		expectErr bool
		validate  func(t *testing.T, modulePath string)
	}{
		{
			name: "create simple module",
			opts: CreateOptions{
				Name: "mycommands",
			},
			expectErr: false,
			validate: func(t *testing.T, modulePath string) {
				t.Helper()
				// Check module directory exists
				info, err := os.Stat(modulePath)
				if err != nil {
					t.Fatalf("module directory not created: %v", err)
				}
				if !info.IsDir() {
					t.Error("module path is not a directory")
				}

				// Check invkmod.cue exists (required)
				invkmodPath := filepath.Join(modulePath, "invkmod.cue")
				if _, statErr := os.Stat(invkmodPath); statErr != nil {
					t.Errorf("invkmod.cue not created: %v", statErr)
				}

				// Check invkfile.cue exists
				invkfilePath := filepath.Join(modulePath, "invkfile.cue")
				if _, statErr := os.Stat(invkfilePath); statErr != nil {
					t.Errorf("invkfile.cue not created: %v", statErr)
				}

				// Verify module is valid
				_, err = Load(modulePath)
				if err != nil {
					t.Errorf("created module is not valid: %v", err)
				}
			},
		},
		{
			name: "create RDNS module",
			opts: CreateOptions{
				Name: "com.example.mytools",
			},
			expectErr: false,
			validate: func(t *testing.T, modulePath string) {
				t.Helper()
				if !strings.HasSuffix(modulePath, "com.example.mytools.invkmod") {
					t.Errorf("unexpected module path: %s", modulePath)
				}
				// Verify invkmod.cue contains correct module ID
				content, err := os.ReadFile(filepath.Join(modulePath, "invkmod.cue"))
				if err != nil {
					t.Fatalf("failed to read invkmod.cue: %v", err)
				}
				if !strings.Contains(string(content), `module: "com.example.mytools"`) {
					t.Error("module ID not set correctly in invkmod.cue")
				}
			},
		},
		{
			name: "create module with scripts directory",
			opts: CreateOptions{
				Name:             "mytools",
				CreateScriptsDir: true,
			},
			expectErr: false,
			validate: func(t *testing.T, modulePath string) {
				t.Helper()
				scriptsDir := filepath.Join(modulePath, "scripts")
				info, err := os.Stat(scriptsDir)
				if err != nil {
					t.Fatalf("scripts directory not created: %v", err)
				}
				if !info.IsDir() {
					t.Error("scripts path is not a directory")
				}

				// Check .gitkeep exists
				gitkeepPath := filepath.Join(scriptsDir, ".gitkeep")
				if _, err := os.Stat(gitkeepPath); err != nil {
					t.Errorf(".gitkeep not created: %v", err)
				}
			},
		},
		{
			name: "create module with custom module identifier",
			opts: CreateOptions{
				Name:   "mytools",
				Module: "custom.module",
			},
			expectErr: false,
			validate: func(t *testing.T, modulePath string) {
				t.Helper()
				// Custom module ID should be in invkmod.cue (not invkfile.cue)
				content, err := os.ReadFile(filepath.Join(modulePath, "invkmod.cue"))
				if err != nil {
					t.Fatalf("failed to read invkmod.cue: %v", err)
				}
				if !strings.Contains(string(content), `module: "custom.module"`) {
					t.Error("custom module not set in invkmod.cue")
				}
			},
		},
		{
			name: "create module with custom description",
			opts: CreateOptions{
				Name:        "mytools",
				Description: "My custom description",
			},
			expectErr: false,
			validate: func(t *testing.T, modulePath string) {
				t.Helper()
				// Description should be in invkmod.cue (not invkfile.cue)
				content, err := os.ReadFile(filepath.Join(modulePath, "invkmod.cue"))
				if err != nil {
					t.Fatalf("failed to read invkmod.cue: %v", err)
				}
				if !strings.Contains(string(content), `description: "My custom description"`) {
					t.Error("custom description not set in invkmod.cue")
				}
			},
		},
		{
			name: "empty name fails",
			opts: CreateOptions{
				Name: "",
			},
			expectErr: true,
		},
		{
			name: "invalid name fails",
			opts: CreateOptions{
				Name: "123invalid",
			},
			expectErr: true,
		},
		{
			name: "name with hyphen fails",
			opts: CreateOptions{
				Name: "my-commands",
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use temp directory as parent
			tmpDir := t.TempDir()
			opts := tt.opts
			opts.ParentDir = tmpDir

			modulePath, err := Create(opts)
			if tt.expectErr {
				if err == nil {
					t.Error("Create() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Create() unexpected error: %v", err)
			}

			if tt.validate != nil {
				tt.validate(t, modulePath)
			}
		})
	}
}

func TestCreate_ExistingModule(t *testing.T) {
	tmpDir := t.TempDir()

	// Create module first time
	opts := CreateOptions{
		Name:      "mytools",
		ParentDir: tmpDir,
	}

	_, err := Create(opts)
	if err != nil {
		t.Fatalf("first Create() failed: %v", err)
	}

	// Try to create again - should fail
	_, err = Create(opts)
	if err == nil {
		t.Error("Create() expected error for existing module, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestArchive(t *testing.T) {
	t.Run("archive valid module", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a module first
		modulePath, err := Create(CreateOptions{
			Name:             "mytools",
			ParentDir:        tmpDir,
			CreateScriptsDir: true,
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		// Add a script file
		scriptPath := filepath.Join(modulePath, "scripts", "test.sh")
		if writeErr := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho hello"), 0o755); writeErr != nil {
			t.Fatalf("failed to write script: %v", writeErr)
		}

		// Archive the module
		outputPath := filepath.Join(tmpDir, "output.zip")
		zipPath, err := Archive(modulePath, outputPath)
		if err != nil {
			t.Fatalf("Archive() failed: %v", err)
		}

		// Verify ZIP was created
		info, err := os.Stat(zipPath)
		if err != nil {
			t.Fatalf("ZIP file not created: %v", err)
		}
		if info.Size() == 0 {
			t.Error("ZIP file is empty")
		}

		// Verify ZIP path matches expected
		if zipPath != outputPath {
			t.Errorf("Archive() returned %q, expected %q", zipPath, outputPath)
		}
	})

	t.Run("archive with default output path", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a module
		modulePath, err := Create(CreateOptions{
			Name:      "com.example.tools",
			ParentDir: tmpDir,
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		// Change to temp dir to test default output
		restoreWd := testutil.MustChdir(t, tmpDir)
		defer restoreWd()

		// Archive with empty output path
		zipPath, err := Archive(modulePath, "")
		if err != nil {
			t.Fatalf("Archive() failed: %v", err)
		}

		// Verify default name
		expectedName := "com.example.tools.invkmod.zip"
		if filepath.Base(zipPath) != expectedName {
			t.Errorf("default ZIP name = %q, expected %q", filepath.Base(zipPath), expectedName)
		}
	})

	t.Run("archive invalid module fails", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create an invalid module (no invkfile)
		modulePath := filepath.Join(tmpDir, "invalid.invkmod")
		if err := os.Mkdir(modulePath, 0o755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		_, err := Archive(modulePath, "")
		if err == nil {
			t.Error("Archive() expected error for invalid module, got nil")
		}
	})
}

func TestUnpack(t *testing.T) {
	t.Run("unpack valid module from ZIP", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create and archive a module
		modulePath, err := Create(CreateOptions{
			Name:      "mytools",
			ParentDir: tmpDir,
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		zipPath := filepath.Join(tmpDir, "module.zip")
		_, err = Archive(modulePath, zipPath)
		if err != nil {
			t.Fatalf("Archive() failed: %v", err)
		}

		// Remove original module
		testutil.MustRemoveAll(t, modulePath)

		// Unpack to a different directory
		unpackDir := filepath.Join(tmpDir, "unpacked")
		if mkdirErr := os.Mkdir(unpackDir, 0o755); mkdirErr != nil {
			t.Fatalf("failed to create unpack dir: %v", mkdirErr)
		}

		extractedPath, err := Unpack(UnpackOptions{
			Source:  zipPath,
			DestDir: unpackDir,
		})
		if err != nil {
			t.Fatalf("Unpack() failed: %v", err)
		}

		// Verify extracted module is valid
		b, err := Load(extractedPath)
		if err != nil {
			t.Fatalf("extracted module is invalid: %v", err)
		}

		if b.Name() != "mytools" {
			t.Errorf("extracted module name = %q, expected %q", b.Name(), "mytools")
		}
	})

	t.Run("unpack fails for existing module without overwrite", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create and archive a module
		modulePath, err := Create(CreateOptions{
			Name:      "mytools",
			ParentDir: tmpDir,
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		zipPath := filepath.Join(tmpDir, "module.zip")
		_, err = Archive(modulePath, zipPath)
		if err != nil {
			t.Fatalf("Archive() failed: %v", err)
		}

		// Try to unpack to same directory (module already exists)
		_, err = Unpack(UnpackOptions{
			Source:    zipPath,
			DestDir:   tmpDir,
			Overwrite: false,
		})
		if err == nil {
			t.Error("Unpack() expected error for existing module, got nil")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("expected 'already exists' error, got: %v", err)
		}
	})

	t.Run("unpack with overwrite replaces existing module", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create and archive a module
		modulePath, err := Create(CreateOptions{
			Name:      "mytools",
			ParentDir: tmpDir,
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		zipPath := filepath.Join(tmpDir, "module.zip")
		_, err = Archive(modulePath, zipPath)
		if err != nil {
			t.Fatalf("Archive() failed: %v", err)
		}

		// Modify the existing module
		markerFile := filepath.Join(modulePath, "marker.txt")
		if writeErr := os.WriteFile(markerFile, []byte("marker"), 0o644); writeErr != nil {
			t.Fatalf("failed to create marker file: %v", writeErr)
		}

		// Unpack with overwrite
		extractedPath, err := Unpack(UnpackOptions{
			Source:    zipPath,
			DestDir:   tmpDir,
			Overwrite: true,
		})
		if err != nil {
			t.Fatalf("Unpack() with overwrite failed: %v", err)
		}

		// Verify marker file is gone (module was replaced)
		if _, statErr := os.Stat(filepath.Join(extractedPath, "marker.txt")); !os.IsNotExist(statErr) {
			t.Error("marker file should not exist after overwrite")
		}
	})

	t.Run("unpack fails for empty source", func(t *testing.T) {
		_, err := Unpack(UnpackOptions{
			Source: "",
		})
		if err == nil {
			t.Error("Unpack() expected error for empty source, got nil")
		}
	})

	t.Run("unpack fails for invalid ZIP", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create an invalid ZIP file
		invalidZip := filepath.Join(tmpDir, "invalid.zip")
		if err := os.WriteFile(invalidZip, []byte("not a zip file"), 0o644); err != nil {
			t.Fatalf("failed to create invalid ZIP: %v", err)
		}

		_, err := Unpack(UnpackOptions{
			Source:  invalidZip,
			DestDir: tmpDir,
		})
		if err == nil {
			t.Error("Unpack() expected error for invalid ZIP, got nil")
		}
	})

	t.Run("unpack fails for ZIP without module", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a ZIP file without a module
		zipPath := filepath.Join(tmpDir, "nomodule.zip")
		zipFile, err := os.Create(zipPath)
		if err != nil {
			t.Fatalf("failed to create ZIP file: %v", err)
		}
		zipWriter := zip.NewWriter(zipFile)
		w, _ := zipWriter.Create("somefile.txt")
		_, _ = w.Write([]byte("content")) // Test setup; error non-critical
		_ = zipWriter.Close()             // Test setup; error non-critical
		_ = zipFile.Close()               // Test setup; error non-critical

		_, err = Unpack(UnpackOptions{
			Source:  zipPath,
			DestDir: tmpDir,
		})
		if err == nil {
			t.Error("Unpack() expected error for ZIP without module, got nil")
		}
		if !strings.Contains(err.Error(), "no valid module found") {
			t.Errorf("expected 'no valid module found' error, got: %v", err)
		}
	})
}

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
