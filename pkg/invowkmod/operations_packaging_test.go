// SPDX-License-Identifier: MPL-2.0

package invowkmod

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invowk/invowk/internal/testutil"
	"github.com/invowk/invowk/pkg/types"
)

// ============================================================================
// Tests for Module Packaging Operations (Archive, Unpack)
// ============================================================================

// Helper function to create a valid module with both invowkmod.cue and invowkfile.cue
func createValidModuleForPackaging(t *testing.T, dir, folderName, moduleID string) string {
	t.Helper()
	modulePath := filepath.Join(dir, folderName)
	if err := os.Mkdir(modulePath, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create invowkmod.cue with metadata
	invowkmodPath := filepath.Join(modulePath, "invowkmod.cue")
	invowkmodContent := fmt.Sprintf(`module: "%s"
version: "1.0.0"
`, moduleID)
	if err := os.WriteFile(invowkmodPath, []byte(invowkmodContent), 0o644); err != nil {
		t.Fatal(err)
	}
	// Create invowkfile.cue with commands
	invowkfilePath := filepath.Join(modulePath, "invowkfile.cue")
	if err := os.WriteFile(invowkfilePath, []byte("cmds: []"), 0o644); err != nil {
		t.Fatal(err)
	}
	return modulePath
}

func TestArchive(t *testing.T) {
	t.Run("archive valid module", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a module first
		modulePath, err := Create(CreateOptions{
			Name:             "mytools",
			ParentDir:        types.FilesystemPath(tmpDir),
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
		zipPath, err := Archive(types.FilesystemPath(modulePath), types.FilesystemPath(outputPath))
		if err != nil {
			t.Fatalf("Archive() failed: %v", err)
		}

		// Verify ZIP was created
		info, err := os.Stat(string(zipPath))
		if err != nil {
			t.Fatalf("ZIP file not created: %v", err)
		}
		if info.Size() == 0 {
			t.Error("ZIP file is empty")
		}

		// Verify ZIP path matches expected
		if string(zipPath) != outputPath {
			t.Errorf("Archive() returned %q, expected %q", zipPath, outputPath)
		}
	})

	t.Run("archive with default output path", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a module
		modulePath, err := Create(CreateOptions{
			Name:      "com.example.tools",
			ParentDir: types.FilesystemPath(tmpDir),
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		// Change to temp dir to test default output
		restoreWd := testutil.MustChdir(t, tmpDir)
		defer restoreWd()

		// Archive with empty output path
		zipPath, err := Archive(types.FilesystemPath(modulePath), "")
		if err != nil {
			t.Fatalf("Archive() failed: %v", err)
		}

		// Verify default name
		expectedName := "com.example.tools.invowkmod.zip"
		if filepath.Base(string(zipPath)) != expectedName {
			t.Errorf("default ZIP name = %q, expected %q", filepath.Base(string(zipPath)), expectedName)
		}
	})

	t.Run("archive invalid module fails", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create an invalid module (no invowkfile)
		modulePath := filepath.Join(tmpDir, "invalid.invowkmod")
		if err := os.Mkdir(modulePath, 0o755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		_, err := Archive(types.FilesystemPath(modulePath), "")
		if err == nil {
			t.Error("Archive() expected error for invalid module, got nil")
		}
	})
}

func TestUnpack(t *testing.T) {
	t.Parallel()

	t.Run("unpack valid module from ZIP", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		// Create and archive a module
		modulePath, err := Create(CreateOptions{
			Name:      "mytools",
			ParentDir: types.FilesystemPath(tmpDir),
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		zipPath := filepath.Join(tmpDir, "module.zip")
		_, err = Archive(types.FilesystemPath(modulePath), types.FilesystemPath(zipPath))
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
			DestDir: types.FilesystemPath(unpackDir),
		})
		if err != nil {
			t.Fatalf("Unpack() failed: %v", err)
		}

		// Verify extracted module is valid
		b, err := Load(types.FilesystemPath(extractedPath))
		if err != nil {
			t.Fatalf("extracted module is invalid: %v", err)
		}

		if b.Name() != "mytools" {
			t.Errorf("extracted module name = %q, expected %q", b.Name(), "mytools")
		}
	})

	t.Run("unpack fails for existing module without overwrite", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		// Create and archive a module
		modulePath, err := Create(CreateOptions{
			Name:      "mytools",
			ParentDir: types.FilesystemPath(tmpDir),
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		zipPath := filepath.Join(tmpDir, "module.zip")
		_, err = Archive(types.FilesystemPath(modulePath), types.FilesystemPath(zipPath))
		if err != nil {
			t.Fatalf("Archive() failed: %v", err)
		}

		// Try to unpack to same directory (module already exists)
		_, err = Unpack(UnpackOptions{
			Source:    zipPath,
			DestDir:   types.FilesystemPath(tmpDir),
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
		t.Parallel()

		tmpDir := t.TempDir()

		// Create and archive a module
		modulePath, err := Create(CreateOptions{
			Name:      "mytools",
			ParentDir: types.FilesystemPath(tmpDir),
		})
		if err != nil {
			t.Fatalf("Create() failed: %v", err)
		}

		zipPath := filepath.Join(tmpDir, "module.zip")
		_, err = Archive(types.FilesystemPath(modulePath), types.FilesystemPath(zipPath))
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
			DestDir:   types.FilesystemPath(tmpDir),
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
		t.Parallel()

		_, err := Unpack(UnpackOptions{
			Source: "",
		})
		if err == nil {
			t.Error("Unpack() expected error for empty source, got nil")
		}
	})

	t.Run("unpack fails for invalid ZIP", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()

		// Create an invalid ZIP file
		invalidZip := filepath.Join(tmpDir, "invalid.zip")
		if err := os.WriteFile(invalidZip, []byte("not a zip file"), 0o644); err != nil {
			t.Fatalf("failed to create invalid ZIP: %v", err)
		}

		_, err := Unpack(UnpackOptions{
			Source:  invalidZip,
			DestDir: types.FilesystemPath(tmpDir),
		})
		if err == nil {
			t.Error("Unpack() expected error for invalid ZIP, got nil")
		}
	})

	t.Run("unpack fails for ZIP without module", func(t *testing.T) {
		t.Parallel()

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
			DestDir: types.FilesystemPath(tmpDir),
		})
		if err == nil {
			t.Error("Unpack() expected error for ZIP without module, got nil")
		}
		if !strings.Contains(err.Error(), "no valid module found") {
			t.Errorf("expected 'no valid module found' error, got: %v", err)
		}
	})
}
