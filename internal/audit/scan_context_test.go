// SPDX-License-Identifier: MPL-2.0

package audit

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/invowk/invowk/internal/config"
	"github.com/invowk/invowk/pkg/invowkmod"
	"github.com/invowk/invowk/pkg/types"
)

func TestReadScriptFileContent_BoundaryCheck(t *testing.T) {
	t.Parallel()

	// Create a module directory with a legitimate script inside it.
	moduleDir := t.TempDir()
	scriptContent := "#!/bin/sh\necho hello"
	if err := os.WriteFile(filepath.Join(moduleDir, "run.sh"), []byte(scriptContent), 0o644); err != nil {
		t.Fatalf("failed to write script: %v", err)
	}

	// Create a sensitive file outside the module boundary.
	outsideDir := t.TempDir()
	sensitiveContent := "SECRET=hunter2"
	if err := os.WriteFile(filepath.Join(outsideDir, "secret.env"), []byte(sensitiveContent), 0o644); err != nil {
		t.Fatalf("failed to write sensitive file: %v", err)
	}

	// Compute a traversal path that escapes the module directory.
	relToSensitive, err := filepath.Rel(moduleDir, filepath.Join(outsideDir, "secret.env"))
	if err != nil {
		t.Fatalf("failed to compute relative path: %v", err)
	}

	tests := []struct {
		name       string
		scriptPath string
		modulePath string
		want       string
	}{
		{
			name:       "legitimate script within module",
			scriptPath: "run.sh",
			modulePath: moduleDir,
			want:       scriptContent,
		},
		{
			name:       "traversal path blocked by boundary check",
			scriptPath: relToSensitive,
			modulePath: moduleDir,
			want:       "",
		},
		{
			name:       "explicit dotdot traversal blocked",
			scriptPath: "../../etc/passwd",
			modulePath: moduleDir,
			want:       "",
		},
		{
			name:       "absolute path without module context",
			scriptPath: filepath.Join(moduleDir, "run.sh"),
			modulePath: "",
			want:       scriptContent,
		},
		{
			name:       "nonexistent file returns empty",
			scriptPath: "nonexistent.sh",
			modulePath: moduleDir,
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := readScriptFileContent(tt.scriptPath, tt.modulePath)
			if got != tt.want {
				t.Errorf("readScriptFileContent(%q, %q) = %q, want %q",
					tt.scriptPath, tt.modulePath, got, tt.want)
			}
		})
	}
}

func TestBuildScanContextIncludedModuleKeepsLockAndVendoredArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	includeDir := filepath.Join(t.TempDir(), "io.example.included.invowkmod")
	createAuditTestModule(t, includeDir, "io.example.included", "included-cmd")
	vendorDir := filepath.Join(includeDir, invowkmod.VendoredModulesDir, "io.example.dep.invowkmod")
	createAuditTestModule(t, vendorDir, "io.example.dep", "dep-cmd")
	hash, err := invowkmod.ComputeModuleHash(vendorDir)
	if err != nil {
		t.Fatalf("ComputeModuleHash() = %v", err)
	}
	lock := invowkmod.NewLockFile()
	lock.Modules["https://example.com/dep.git"] = invowkmod.LockedModule{
		GitURL:          "https://example.com/dep.git",
		Version:         "^1.0.0",
		ResolvedVersion: "1.0.0",
		GitCommit:       "0123456789abcdef0123456789abcdef01234567",
		Namespace:       "io.example.dep",
		ModuleID:        "io.example.dep",
		ContentHash:     hash,
	}
	if saveErr := lock.Save(filepath.Join(includeDir, invowkmod.LockFileName)); saveErr != nil {
		t.Fatalf("lock.Save() = %v", saveErr)
	}
	if writeErr := os.WriteFile(filepath.Join(vendorDir, "tampered.txt"), []byte("changed after lock"), 0o644); writeErr != nil {
		t.Fatalf("write tampered file: %v", writeErr)
	}

	cfg := config.DefaultConfig()
	cfg.Includes = []config.IncludeEntry{{Path: config.ModuleIncludePath(includeDir)}}
	sc, err := BuildScanContext(types.FilesystemPath(root), cfg, false)
	if err != nil {
		t.Fatalf("BuildScanContext() = %v", err)
	}

	var included *ScannedModule
	for _, mod := range sc.Modules() {
		if mod.SurfaceID == "io.example.included" {
			included = mod
			break
		}
	}
	if included == nil {
		t.Fatalf("included module not scanned; modules: %v", sc.Modules())
	}
	if included.LockFile == nil {
		t.Fatal("included LockFile = nil, want loaded lock file")
	}
	if len(included.VendoredModules) != 1 {
		t.Fatalf("included VendoredModules = %d, want 1", len(included.VendoredModules))
	}
}

func createAuditTestModule(t *testing.T, moduleDir, moduleID, cmdName string) {
	t.Helper()
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatalf("mkdir module: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkmod.cue"), []byte(`module: "`+moduleID+`"
version: "1.0.0"
`), 0o644); err != nil {
		t.Fatalf("write invowkmod.cue: %v", err)
	}
	if err := os.WriteFile(filepath.Join(moduleDir, "invowkfile.cue"), []byte(`cmds: [{
	name: "`+cmdName+`"
	implementations: [{
		script: "echo test"
		runtimes: [{name: "native"}]
		platforms: [{name: "linux"}, {name: "macos"}]
	}]
}]
`), 0o644); err != nil {
		t.Fatalf("write invowkfile.cue: %v", err)
	}
}
